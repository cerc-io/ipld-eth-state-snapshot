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
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/statediff/indexer/postgres"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/jmoiron/sqlx"
	"github.com/sirupsen/logrus"

	iter "github.com/vulcanize/go-eth-state-node-iterator"
)

var (
	nullHash          = common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000000")
	emptyNode, _      = rlp.EncodeToBytes([]byte{})
	emptyCodeHash     = crypto.Keccak256([]byte{})
	emptyContractRoot = crypto.Keccak256Hash(emptyNode)

	defaultBatchSize = uint(100)
)

// Service holds ethDB and stateDB to read data from lvldb and Publisher
// to publish trie in postgres DB.
type Service struct {
	ethDB         ethdb.Database
	stateDB       state.Database
	ipfsPublisher *Publisher
	maxBatchSize  uint
}

// NewSnapshotService creates Service.
func NewSnapshotService(con *Config) (*Service, error) {
	pgDB, err := postgres.NewDB(con.connectionURI, con.DBConfig, con.Node)
	if err != nil {
		return nil, err
	}

	edb, err := rawdb.NewLevelDBDatabaseWithFreezer(con.LevelDBPath, 1024, 256, con.AncientDBPath, "eth-pg-ipfs-state-snapshot", false)
	if err != nil {
		return nil, err
	}

	return &Service{
		ethDB:         edb,
		stateDB:       state.NewDatabase(edb),
		ipfsPublisher: NewPublisher(pgDB),
		maxBatchSize:  defaultBatchSize,
	}, nil
}

type SnapshotParams struct {
	Height  uint64
	Workers uint
}

func (s *Service) CreateSnapshot(params SnapshotParams) error {
	// extract header from lvldb and publish to PG-IPFS
	// hold onto the headerID so that we can link the state nodes to this header
	logrus.Infof("Creating snapshot at height %d", params.Height)
	hash := rawdb.ReadCanonicalHash(s.ethDB, params.Height)
	header := rawdb.ReadHeader(s.ethDB, hash, params.Height)
	if header == nil {
		return fmt.Errorf("unable to read canonical header at height %d", params.Height)
	}

	logrus.Infof("head hash: %s head height: %d", hash.Hex(), params.Height)

	headerID, err := s.ipfsPublisher.PublishHeader(header)
	if err != nil {
		return err
	}

	t, err := s.stateDB.OpenTrie(header.Root)
	if err != nil {
		return err
	}
	if params.Workers > 0 {
		return s.createSnapshotAsync(t, headerID, params.Workers)
	} else {
		return s.createSnapshot(t.NodeIterator(nil), headerID)
	}
	return nil
}

// Create snapshot up to head (ignores height param)
func (s *Service) CreateLatestSnapshot(workers uint) error {
	logrus.Info("Creating snapshot at head")
	hash := rawdb.ReadHeadHeaderHash(s.ethDB)
	height := rawdb.ReadHeaderNumber(s.ethDB, hash)
	if height == nil {
		return fmt.Errorf("unable to read header height for header hash %s", hash.String())
	}
	return s.CreateSnapshot(SnapshotParams{Height: *height, Workers: workers})
}

type nodeResult struct {
	node     Node
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

func (s *Service) createSnapshot(it trie.NodeIterator, headerID int64) error {
	for it.Next(true) {
		if it.Leaf() { // "leaf" nodes are actually "value" nodes, whose parents are the actual leaves
			return nil
		}

		if bytes.Equal(nullHash.Bytes(), it.Hash().Bytes()) {
			return nil
		}
		tx, err = s.ipfsPublisher.checkBatchSize(tx, s.maxBatchSize)
		if err != nil {
			return err
		}

		res, err := resolveNode(it, s.stateDB.TrieDB())
		if err != nil {
			return err
		}
		switch res.node.NodeType {
		case leaf:
			// if the node is a leaf, decode the account and publish the associated storage trie nodes if there are any
			// var account snapshot.Account
			var account types.StateAccount
			if err := rlp.DecodeBytes(res.elements[1].([]byte), &account); err != nil {
				return fmt.Errorf(
					"error decoding account for leaf node at path %x nerror: %v", res.node.Path, err)
			}
			partialPath := trie.CompactToHex(res.elements[0].([]byte))
			valueNodePath := append(res.node.Path, partialPath...)
			encodedPath := trie.HexToCompact(valueNodePath)
			leafKey := encodedPath[1:]
			res.node.Key = common.BytesToHash(leafKey)
			stateID, err := s.ipfsPublisher.PublishStateNode(res.node, headerID)
			if err != nil {
				return err
			}

			// publish any non-nil code referenced by codehash
			if !bytes.Equal(account.CodeHash, emptyCodeHash) {
				codeHash := common.BytesToHash(account.CodeHash)
				codeBytes := rawdb.ReadCode(s.ethDB, codeHash)
				if len(codeBytes) == 0 {
					logrus.Error("Code is missing", "account", common.BytesToHash(it.LeafKey()))
					return errors.New("missing code")
				}

				if err = s.ipfsPublisher.PublishCode(codeHash, codeBytes, tx); err != nil {
					return err
				}
			}

			if tx, err = s.storageSnapshot(account.Root, stateID, tx); err != nil {
				return fmt.Errorf("failed building storage snapshot for account %+v\r\nerror: %w", account, err)
			}
		case extension, branch:
			stateNode.key = common.BytesToHash([]byte{})
			if _, err := s.ipfsPublisher.PublishStateNode(stateNode, headerID, tx); err != nil {
				return err
			}
		default:
			return errors.New("unexpected node type")
		}
		return nil

	}
	return it.Error()
}

// Full-trie concurrent snapshot
func (s *Service) createSnapshotAsync(tree state.Trie, headerID int64, workers uint) error {
	errors := make(chan error)
	var wg sync.WaitGroup
	for _, it := range iter.SubtrieIterators(tree, workers) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.createSnapshot(it, headerID); err != nil {
				errors <- err
			}
		}()
	}
	go func() {
		defer close(errors)
		wg.Wait()
	}()

	select {
	case err := <-errors:
		return err
	}
	return nil
}

func (s *Service) storageSnapshot(sr common.Hash, stateID int64, tx *sqlx.Tx) (*sqlx.Tx, error) {
	if bytes.Equal(sr.Bytes(), emptyContractRoot.Bytes()) {
		return tx, nil
	}

	sTrie, err := s.stateDB.OpenTrie(sr)
	if err != nil {
		return nil, err
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
			return nil, err
		}

		tx, err = s.ipfsPublisher.checkBatchSize(tx, s.maxBatchSize)
		if err != nil {
			return nil, err
		}

		nodeData, err = s.stateDB.TrieDB().Node(it.Hash())
		if err != nil {
			return nil, err
		}
		switch res.node.NodeType {
		case leaf:
			partialPath := trie.CompactToHex(res.elements[0].([]byte))
			valueNodePath := append(res.node.Path, partialPath...)
			encodedPath := trie.HexToCompact(valueNodePath)
			leafKey := encodedPath[1:]
			res.node.Key = common.BytesToHash(leafKey)
		case extension, branch:
			res.node.Key = common.BytesToHash([]byte{})
		default:
			return nil, errors.New("unexpected node type")
		}
		if err = s.ipfsPublisher.PublishStorageNode(res.node, stateID); err != nil {
			return err
		}
	}

	return tx, it.Error()
}
