// Copyright © 2020 Vulcanize, Inc
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package snapshot

import (
	"bytes"
	"errors"
	"fmt"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/sirupsen/logrus"

	"github.com/vulcanize/ipfs-blockchain-watcher/pkg/postgres"
)

var (
	nullHash          = common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000000")
	emptyNode, _      = rlp.EncodeToBytes([]byte{})
	emptyCodeHash     = crypto.Keccak256([]byte{})
	emptyContractRoot = crypto.Keccak256Hash(emptyNode)
	spawnDepth		  = 2
)

type Service struct {
	ethDB         ethdb.Database
	stateDB       state.Database
	ipfsPublisher *Publisher
}


func NewSnapshotService(con ServiceConfig) (*Service, error) {
	pgdb, err := postgres.NewDB(con.DBConfig, con.Node)
	if err != nil {
		return nil, err
	}
	edb, err := rawdb.NewLevelDBDatabaseWithFreezer(con.LevelDBPath, 1024, 256, con.AncientDBPath, "eth-pg-ipfs-state-snapshot")
	if err != nil {
		return nil, err
	}
	return &Service{
		ethDB:         edb,
		stateDB:       state.NewDatabase(edb),
		ipfsPublisher: NewPublisher(pgdb),
	}, nil
}

type SnapshotParams struct {
	Height uint64
	Async bool
}

type workerParams struct {
	trieDB *trie.Database
	headerID int64
	waitGroup *sync.WaitGroup
}

func (s *Service) CreateSnapshot(par SnapshotParams) error {
	// extract header from lvldb and publish to PG-IPFS
	// hold onto the headerID so that we can link the state nodes to this header
	logrus.Infof("Creating snapshot at height %d", par.Height)
	hash := rawdb.ReadCanonicalHash(s.ethDB, par.Height)
	header := rawdb.ReadHeader(s.ethDB, hash, par.Height)
	if header == nil {
		return fmt.Errorf("unable to read canonical header at height %d", par.Height)
	}
	headerID, err := s.ipfsPublisher.PublishHeader(header)
	if err != nil {
		return err
	}
	t, err := s.stateDB.OpenTrie(header.Root)
	if err != nil {
		return err
	}
	trieDB := s.stateDB.TrieDB()
	params := workerParams{trieDB: trieDB, headerID: headerID}
	if par.Async {
		var wg sync.WaitGroup
		params.waitGroup = &wg
		return s.createSnapshotAsync(t.NodeIterator([]byte{}), 0, params)
		wg.Wait()
	} else {
		return s.createSnapshot(t.NodeIterator([]byte{}), params)
	}
	return nil
}

// Create snapshot up to head (ignores height param)
func (s *Service) CreateLatestSnapshot(doAsync bool) error {
	logrus.Info("Creating snapshot at head")
	hash := rawdb.ReadHeadHeaderHash(s.ethDB)
	height := rawdb.ReadHeaderNumber(s.ethDB, hash)
	if height == nil {
		return fmt.Errorf("unable to read header height for header hash %s", hash.String())
	}
	return s.CreateSnapshot(SnapshotParams{Height: *height, Async: doAsync})
}

// cache the elements
type nodeResult struct {
	node Node
	elements []interface{}
}

func resolveNode(it trie.NodeIterator, trieDB *trie.Database) (*nodeResult, error) {
	nodePath := make([]byte, len(it.Path()))
	copy(nodePath, it.Path())
	node, err := trieDB.Node(it.Hash())
	if err != nil {
		return nil, err
	}
	var nodeElements []interface{}
	if err := rlp.DecodeBytes(node, &nodeElements); err != nil {
		return nil, err
	}
	ty, err := CheckKeyType(nodeElements)
	if err != nil {
		return nil, err
	}
	return &nodeResult{
		node: Node{
			NodeType: ty,
			Path:     nodePath,
			Value:    node,
		},
		elements: nodeElements,
	}, nil
}


func (s *Service) processNode(it trie.NodeIterator, par workerParams) error {
	fmt.Printf("    [!]: %v\n", it.Path())

	if it.Leaf() { // "leaf" nodes are actually "value" nodes, whose parents are the actual leaves
		return nil
	}
	if bytes.Equal(nullHash.Bytes(), it.Hash().Bytes()) {
		return nil
	}
	res, err := resolveNode(it, par.trieDB)
	if err != nil {
		return err
	}
	switch res.node.NodeType {
	case Leaf:
		// if the node is a leaf, decode the account and publish the associated storage trie nodes if there are any
		var account state.Account
		if err := rlp.DecodeBytes(res.elements[1].([]byte), &account); err != nil {
			return fmt.Errorf(
				"error decoding account for leaf node at path %x nerror: %v", res.node.Path, err)
		}
		partialPath := trie.CompactToHex(res.elements[0].([]byte))
		valueNodePath := append(res.node.Path, partialPath...)
		encodedPath := trie.HexToCompact(valueNodePath)
		leafKey := encodedPath[1:]
		res.node.Key = common.BytesToHash(leafKey)
		stateID, err := s.ipfsPublisher.PublishStateNode(res.node, par.headerID)
		if err != nil {
			return err
		}
		// publish any non-nil code referenced by codehash
		if !bytes.Equal(account.CodeHash, emptyCodeHash) {
			codeBytes, err := s.ethDB.Get(account.CodeHash)
			if err != nil {
				return err
			}
			if err := s.ipfsPublisher.PublishCode(codeBytes); err != nil {
				return err
			}
		}
		if err := s.storageSnapshot(account.Root, stateID); err != nil {
			return fmt.Errorf("failed building storage snapshot for account %+v\r\nerror: %v", account, err)
		}
	case Extension, Branch:
		res.node.Key = common.BytesToHash([]byte{})
		if _, err := s.ipfsPublisher.PublishStateNode(res.node, par.headerID); err != nil {
			return err
		}
	default:
		return errors.New("unexpected node type")
	}
	return nil
}

func (s *Service) createSnapshot(it trie.NodeIterator, par workerParams) error {
	if par.waitGroup != nil {
		defer par.waitGroup.Done()
	}
	for it.Next(true) {
		if err := s.processNode(it, par); err != nil {
			return err
		}
	}
	return it.Error()
}

// Iterate sequentially up to spawnDepth, then create workers to snapshot the subtrees
func (s *Service) processNodeAsync(it trie.NodeIterator, depth int, par workerParams) error {
	fmt.Printf("  [&, %v]: %v\n", depth, it.Path())

	if it.Leaf() { // "leaf" nodes are actually "value" nodes, whose parents are the actual leaves
		return nil
	}
	if bytes.Equal(nullHash.Bytes(), it.Hash().Bytes()) {
		return nil
	}
	res, err := resolveNode(it, par.trieDB)
	if err != nil {
		return err
	}
	switch res.node.NodeType {
	case Leaf:
		depth = spawnDepth		// if it's a leaf, just spawn early
	case Extension, Branch:
		depth += 1
	default:
		return errors.New("unexpected node type")
	}

	// get a new iterator for the child
	// XXX
	t, err := s.stateDB.OpenTrie(it.Hash())
	return s.createSnapshotAsync(t.NodeIterator(nil), depth, par)
}

func (s *Service) createSnapshotAsync(it trie.NodeIterator, depth int, par workerParams) error {
	if depth >= spawnDepth {
		print("  worker handoff\n")
		par.waitGroup.Add(1)
		go s.createSnapshot(it, par)
		return nil
	}

	// shallow traversal so we can control the depth (?)
	for it.Next(true) {
		err := s.processNodeAsync(it, depth, par)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) storageSnapshot(sr common.Hash, stateID int64) error {
	if bytes.Equal(sr.Bytes(), emptyContractRoot.Bytes()) {
		return nil
	}
	sTrie, err := s.stateDB.OpenTrie(sr)
	if err != nil {
		return err
	}
	it := sTrie.NodeIterator(make([]byte, 0))
	for it.Next(true) {
		// skip value nodes
		if it.Leaf() {
			continue
		}
		if bytes.Equal(nullHash.Bytes(), it.Hash().Bytes()) {
			continue
		}
		res, err := resolveNode(it, s.stateDB.TrieDB())
		if err != nil {
			return err
		}
		switch res.node.NodeType {
		case Leaf:
			partialPath := trie.CompactToHex(res.elements[0].([]byte))
			valueNodePath := append(res.node.Path, partialPath...)
			encodedPath := trie.HexToCompact(valueNodePath)
			leafKey := encodedPath[1:]
			res.node.Key = common.BytesToHash(leafKey)
		case Extension, Branch:
			res.node.Key = common.BytesToHash([]byte{})
		default:
			return errors.New("unexpected node type")
		}
		if err := s.ipfsPublisher.PublishStorageNode(res.node, stateID); err != nil {
			return err
		}
	}
	return it.Error()
}
