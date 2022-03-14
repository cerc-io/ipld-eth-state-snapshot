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
	"github.com/ethereum/go-ethereum/trie"
	log "github.com/sirupsen/logrus"

	. "github.com/vulcanize/eth-pg-ipfs-state-snapshot/pkg/types"
	iter "github.com/vulcanize/go-eth-state-node-iterator"
)

var (
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
	ipfsPublisher Publisher
	maxBatchSize  uint
	tracker       iteratorTracker
	recoveryFile  string
}

func NewLevelDB(con *EthConfig) (ethdb.Database, error) {
	return rawdb.NewLevelDBDatabaseWithFreezer(
		con.LevelDBPath, 1024, 256, con.AncientDBPath, "eth-pg-ipfs-state-snapshot", false,
	)
}

// NewSnapshotService creates Service.
func NewSnapshotService(edb ethdb.Database, pub Publisher, recoveryFile string) (*Service, error) {
	return &Service{
		ethDB:         edb,
		stateDB:       state.NewDatabase(edb),
		ipfsPublisher: pub,
		maxBatchSize:  defaultBatchSize,
		recoveryFile:  recoveryFile,
	}, nil
}

type SnapshotParams struct {
	Height  uint64
	Workers uint
}

func (s *Service) CreateSnapshot(params SnapshotParams) error {
	// extract header from lvldb and publish to PG-IPFS
	// hold onto the headerID so that we can link the state nodes to this header
	log.Infof("Creating snapshot at height %d", params.Height)
	hash := rawdb.ReadCanonicalHash(s.ethDB, params.Height)
	header := rawdb.ReadHeader(s.ethDB, hash, params.Height)
	if header == nil {
		return fmt.Errorf("unable to read canonical header at height %d", params.Height)
	}

	log.Infof("head hash: %s head height: %d", hash.Hex(), params.Height)

	err := s.ipfsPublisher.PublishHeader(header)
	if err != nil {
		return err
	}

	tree, err := s.stateDB.OpenTrie(header.Root)
	if err != nil {
		return err
	}
	headerID := header.Hash().String()
	s.tracker = newTracker(int(params.Workers))
	go s.tracker.run()

	var iters []trie.NodeIterator
	// attempt to restore from recovery file if it exists
	iters, err = s.tracker.restore(tree, s.recoveryFile)
	if err != nil {
		return err
	}
	if iters != nil {
		if params.Workers < uint(len(iters)) {
			return fmt.Errorf(
				"number of recovered workers (%d) is greater than number configured (%d)",
				len(iters), params.Workers,
			)
		}
	} else { // nothing to restore
		if params.Workers > 1 {
			iters = iter.SubtrieIterators(tree, params.Workers)
		} else {
			iters = []trie.NodeIterator{tree.NodeIterator(nil)}
		}
		for i, it := range iters {
			iters[i] = s.tracker.tracked(it)
		}
	}

	defer func() {
		err := s.tracker.haltAndDump(s.recoveryFile)
		if err != nil {
			log.Error("failed to write recovery file: ", err)
		}
	}()

	if len(iters) > 0 {
		return s.createSnapshotAsync(iters, headerID)
	} else {
		return s.createSnapshot(iters[0], headerID)
	}
	return nil
}

// Create snapshot up to head (ignores height param)
func (s *Service) CreateLatestSnapshot(workers uint) error {
	log.Info("Creating snapshot at head")
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
	// "leaf" nodes are actually "value" nodes, whose parents are the actual leaves
	if it.Leaf() {
		return nil, nil
	}
	if IsNullHash(it.Hash()) {
		return nil, nil
	}

	path := make([]byte, len(it.Path()))
	copy(path, it.Path())
	n, err := trieDB.Node(it.Hash())
	if err != nil {
		return nil, err
	}
	var elements []interface{}
	if err := rlp.DecodeBytes(n, &elements); err != nil {
		return nil, err
	}
	ty, err := CheckKeyType(elements)
	if err != nil {
		return nil, err
	}
	return &nodeResult{
		node: Node{
			NodeType: ty,
			Path:     path,
			Value:    n,
		},
		elements: elements,
	}, nil
}

func (s *Service) createSnapshot(it trie.NodeIterator, headerID string) error {
	tx, err := s.ipfsPublisher.BeginTx()
	if err != nil {
		return err
	}
	defer func() { err = CommitOrRollback(tx, err) }()

	for it.Next(true) {
		res, err := resolveNode(it, s.stateDB.TrieDB())
		if err != nil {
			return err
		}
		if res == nil {
			continue
		}

		tx, err = s.ipfsPublisher.PrepareTxForBatch(tx, s.maxBatchSize)
		if err != nil {
			return err
		}

		switch res.node.NodeType {
		case Leaf:
			// if the node is a leaf, decode the account and publish the associated storage trie
			// nodes if there are any
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
			err := s.ipfsPublisher.PublishStateNode(&res.node, headerID, tx)
			if err != nil {
				return err
			}

			// publish any non-nil code referenced by codehash
			if !bytes.Equal(account.CodeHash, emptyCodeHash) {
				codeHash := common.BytesToHash(account.CodeHash)
				codeBytes := rawdb.ReadCode(s.ethDB, codeHash)
				if len(codeBytes) == 0 {
					log.Error("Code is missing", "account", common.BytesToHash(it.LeafKey()))
					return errors.New("missing code")
				}

				if err = s.ipfsPublisher.PublishCode(codeHash, codeBytes, tx); err != nil {
					return err
				}
			}

			if tx, err = s.storageSnapshot(account.Root, headerID, res.node.Path, tx); err != nil {
				return fmt.Errorf("failed building storage snapshot for account %+v\r\nerror: %w", account, err)
			}
		case Extension, Branch:
			res.node.Key = common.BytesToHash([]byte{})
			if err := s.ipfsPublisher.PublishStateNode(&res.node, headerID, tx); err != nil {
				return err
			}
		default:
			return errors.New("unexpected node type")
		}
	}
	return it.Error()
}

// Full-trie concurrent snapshot
func (s *Service) createSnapshotAsync(iters []trie.NodeIterator, headerID string) error {
	errors := make(chan error)
	var wg sync.WaitGroup
	for _, it := range iters {
		wg.Add(1)
		go func(it trie.NodeIterator) {
			defer wg.Done()
			if err := s.createSnapshot(it, headerID); err != nil {
				errors <- err
			}
		}(it)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		done <- struct{}{}
	}()

	var err error
	select {
	case err = <-errors:
	case <-done:
		close(errors)
	}
	return err
}

func (s *Service) storageSnapshot(sr common.Hash, headerID string, statePath []byte, tx Tx) (Tx, error) {
	if bytes.Equal(sr.Bytes(), emptyContractRoot.Bytes()) {
		return tx, nil
	}

	sTrie, err := s.stateDB.OpenTrie(sr)
	if err != nil {
		return nil, err
	}

	it := sTrie.NodeIterator(make([]byte, 0))
	for it.Next(true) {
		res, err := resolveNode(it, s.stateDB.TrieDB())
		if err != nil {
			return nil, err
		}
		if res == nil {
			continue
		}

		tx, err = s.ipfsPublisher.PrepareTxForBatch(tx, s.maxBatchSize)
		if err != nil {
			return nil, err
		}

		var nodeData []byte
		nodeData, err = s.stateDB.TrieDB().Node(it.Hash())
		if err != nil {
			return nil, err
		}
		res.node.Value = nodeData

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
			return nil, errors.New("unexpected node type")
		}
		if err = s.ipfsPublisher.PublishStorageNode(&res.node, headerID, statePath, tx); err != nil {
			return nil, err
		}
	}

	return tx, it.Error()
}
