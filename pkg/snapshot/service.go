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
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"

	iter "github.com/vulcanize/go-eth-state-node-iterator"
	"github.com/vulcanize/ipld-eth-state-snapshot/pkg/prom"
	. "github.com/vulcanize/ipld-eth-state-snapshot/pkg/types"
)

var (
	emptyNode, _      = rlp.EncodeToBytes(&[]byte{})
	emptyCodeHash     = crypto.Keccak256([]byte{})
	emptyContractRoot = crypto.Keccak256Hash(emptyNode)

	defaultBatchSize = uint(100)
)

// Service holds ethDB and stateDB to read data from lvldb and Publisher
// to publish trie in postgres DB.
type Service struct {
	watchingAddresses bool
	ethDB             ethdb.Database
	stateDB           state.Database
	ipfsPublisher     Publisher
	maxBatchSize      uint
	tracker           iteratorTracker
	recoveryFile      string
}

func NewLevelDB(con *EthConfig) (ethdb.Database, error) {
	edb, err := rawdb.NewLevelDBDatabaseWithFreezer(
		con.LevelDBPath, 1024, 256, con.AncientDBPath, "ipld-eth-state-snapshot", true,
	)
	if err != nil {
		return nil, fmt.Errorf("unable to create NewLevelDBDatabaseWithFreezer: %s", err)
	}
	return edb, nil
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
	WatchedAddresses map[common.Address]struct{}
	Height           uint64
	Workers          uint
}

func (s *Service) CreateSnapshot(params SnapshotParams) error {
	paths := make([][]byte, 0, len(params.WatchedAddresses))
	for addr := range params.WatchedAddresses {
		paths = append(paths, keybytesToHex(crypto.Keccak256(addr.Bytes())))
	}
	s.watchingAddresses = len(paths) > 0
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
	s.tracker = newTracker(s.recoveryFile, int(params.Workers))
	s.tracker.captureSignal()

	var iters []trie.NodeIterator
	// attempt to restore from recovery file if it exists
	iters, err = s.tracker.restore(tree, s.stateDB)
	if err != nil {
		log.Errorf("restore error: %s", err.Error())
		return err
	}

	if iters != nil {
		log.Debugf("restored iterators; count: %d", len(iters))
		if params.Workers < uint(len(iters)) {
			return fmt.Errorf(
				"number of recovered workers (%d) is greater than number configured (%d)",
				len(iters), params.Workers,
			)
		}
	} else { // nothing to restore
		log.Debugf("no iterators to restore")
		if params.Workers > 1 {
			iters = iter.SubtrieIterators(tree, params.Workers)
		} else {
			iters = []trie.NodeIterator{tree.NodeIterator(nil)}
		}
		for i, it := range iters {
			iters[i] = s.tracker.tracked(it, nil)
		}
	}

	defer func() {
		err := s.tracker.haltAndDump()
		if err != nil {
			log.Errorf("failed to write recovery file: %v", err)
		}
	}()

	switch {
	case len(iters) > 1:
		return s.createSnapshotAsync(iters, headerID, new(big.Int).SetUint64(params.Height), paths)
	case len(iters) == 1:
		return s.createSnapshot(context.Background(), iters[0], headerID, new(big.Int).SetUint64(params.Height), paths)
	default:
		return nil
	}
}

// Create snapshot up to head (ignores height param)
func (s *Service) CreateLatestSnapshot(workers uint, watchedAddresses map[common.Address]struct{}) error {
	log.Info("Creating snapshot at head")
	hash := rawdb.ReadHeadHeaderHash(s.ethDB)
	height := rawdb.ReadHeaderNumber(s.ethDB, hash)
	if height == nil {
		return fmt.Errorf("unable to read header height for header hash %s", hash.String())
	}
	return s.CreateSnapshot(SnapshotParams{Height: *height, Workers: workers, WatchedAddresses: watchedAddresses})
}

type nodeResult struct {
	node     Node
	elements []interface{}
}

func resolveNode(nodePath []byte, it trie.NodeIterator, trieDB *trie.Database) (*nodeResult, error) {
	// "leaf" nodes are actually "value" nodes, whose parents are the actual leaves
	if it.Leaf() {
		return nil, nil
	}
	if IsNullHash(it.Hash()) {
		return nil, nil
	}

	path := make([]byte, len(nodePath))
	copy(path, nodePath)
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

func validPath(currentPath []byte, seekingPaths [][]byte) bool {
	for _, seekingPath := range seekingPaths {
		if bytes.HasPrefix(seekingPath, currentPath) {
			return true
		}
	}
	return false
}

func (s *Service) createSnapshot(ctx context.Context, it trie.NodeIterator, headerID string, height *big.Int, seekingPaths [][]byte) error {
	tx, err := s.ipfsPublisher.BeginTx()
	if err != nil {
		return err
	}
	defer func() {
		err = CommitOrRollback(tx, err)
		if err != nil {
			log.Errorf("CommitOrRollback failed: %s", err)
		}
	}()

	// path to be seeked (from recovery dump)
	var recoveredPath []byte
	// latest path seeked from the concurrent iterator
	var seekedPath *[]byte
	// end path for the concurrent iterator
	var endPath []byte

	if iter, ok := it.(*trackedIter); ok {
		seekedPath = &iter.seekedPath
		recoveredPath = append(recoveredPath, *seekedPath...)
		endPath = iter.endPath
	} else {
		return errors.New("untracked iterator")
	}

	return s.createSubTrieSnapshot(ctx, tx, nil, it, recoveredPath, seekedPath, endPath, headerID, height, seekingPaths)
}

func (s *Service) createSubTrieSnapshot(ctx context.Context, tx Tx, prefixPath []byte, subTrieIt trie.NodeIterator, recoveredPath []byte, seekedPath *[]byte, endPath []byte, headerID string, height *big.Int, seekingPaths [][]byte) error {
	prom.IncActiveIterCount()
	defer prom.DecActiveIterCount()

	// descend in the first loop iteration to reach first child node
	descend := true
	for {
		select {
		case <-ctx.Done():
			return errors.New("ctx cancelled")
		default:
			if ok := subTrieIt.Next(descend); !ok {
				return subTrieIt.Error()
			}

			// to avoid descending further
			descend = false

			// move on to next node when path is empty
			if bytes.Equal(subTrieIt.Path(), []byte{}) {
				// if node path is empty and prefix is nil, it's a root node
				if prefixPath == nil {
					// create snapshot of node, if it is a leaf this will also create snapshot of entire storage trie
					if err := s.createNodeSnapshot(tx, subTrieIt.Path(), subTrieIt, headerID, height); err != nil {
						return err
					}
					updateSeekedPath(seekedPath, subTrieIt.Path())
				}

				if ok := subTrieIt.Next(true); !ok {
					return subTrieIt.Error()
				}
			}

			// create the full node path as it.Path() doesn't include the path before subtrie root
			nodePath := append(prefixPath, subTrieIt.Path()...)

			// check iterator upper bound before processing the node
			if !checkUpperPathBound(nodePath, endPath) {
				// explicity stop the iterator in tracker
				if trackedSubtrieIt, ok := subTrieIt.(*trackedIter); ok {
					s.tracker.stopIter(trackedSubtrieIt)
				}
				return subTrieIt.Error()
			}

			// skip if node is before recovered path and not on the recovered path
			if bytes.Compare(recoveredPath, nodePath) > 0 && !(len(nodePath) <= len(recoveredPath) && bytes.Equal(recoveredPath[:len(nodePath)], nodePath)) {
				continue
			}

			// ignore node if it is not along paths of interest
			if s.watchingAddresses && !validPath(nodePath, seekingPaths) {
				// update seeked path since this node is getting ignored
				updateSeekedPath(seekedPath, nodePath)
				// move on to the next node
				continue
			}

			// if the node is along paths of interest
			// create snapshot of node, if it is a leaf this will also create snapshot of entire storage trie
			if err := s.createNodeSnapshot(tx, nodePath, subTrieIt, headerID, height); err != nil {
				return err
			}
			// update seeked path after node has been processed
			updateSeekedPath(seekedPath, nodePath)

			// traverse and process the next level of this subTrie
			nextSubTrieIt, err := s.createSubTrieIt(nodePath, subTrieIt.Hash(), recoveredPath)
			if err != nil {
				return err
			}
			if err := s.createSubTrieSnapshot(ctx, tx, nodePath, nextSubTrieIt, recoveredPath, seekedPath, endPath, headerID, height, seekingPaths); err != nil {
				return err
			}
		}
	}
}

func (s *Service) createSubTrieIt(prefixPath []byte, hash common.Hash, recoveredPath []byte) (trie.NodeIterator, error) {
	// skip to the node from recovered path at this level
	// if node path is behind recovered path
	// and recovered path is greater in length than parent path
	// and recovered path includes the prefix
	var startPath []byte
	if bytes.Compare(recoveredPath, prefixPath) > 0 &&
		len(recoveredPath) > len(prefixPath) &&
		bytes.Equal(recoveredPath[:len(prefixPath)], prefixPath) {
		startPath = append(startPath, recoveredPath[len(prefixPath):len(prefixPath)+1]...)
		// Force the lower bound path to an even length
		if len(startPath)&0b1 == 1 {
			decrementPath(startPath) // decrement first to avoid skipped nodes
			startPath = append(startPath, 0)
		}
	}

	// create subTrie iterator with the given hash
	subTrie, err := s.stateDB.OpenTrie(hash)
	if err != nil {
		return nil, err
	}

	return subTrie.NodeIterator(iter.HexToKeyBytes(startPath)), nil
}

func (s *Service) createNodeSnapshot(tx Tx, path []byte, it trie.NodeIterator, headerID string, height *big.Int) error {
	res, err := resolveNode(path, it, s.stateDB.TrieDB())
	if err != nil {
		return err
	}
	if res == nil {
		return nil
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
		if err := s.ipfsPublisher.PublishStateNode(&res.node, headerID, height, tx); err != nil {
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

			if err = s.ipfsPublisher.PublishCode(height, codeHash, codeBytes, tx); err != nil {
				return err
			}
		}

		if _, err = s.storageSnapshot(account.Root, headerID, height, res.node.Path, tx); err != nil {
			return fmt.Errorf("failed building storage snapshot for account %+v\r\nerror: %w", account, err)
		}
	case Extension, Branch:
		res.node.Key = common.BytesToHash([]byte{})
		if err := s.ipfsPublisher.PublishStateNode(&res.node, headerID, height, tx); err != nil {
			return err
		}
	default:
		return errors.New("unexpected node type")
	}
	return it.Error()
}

// Full-trie concurrent snapshot
func (s *Service) createSnapshotAsync(iters []trie.NodeIterator, headerID string, height *big.Int, seekingPaths [][]byte) error {
	g, ctx := errgroup.WithContext(context.Background())
	for _, it := range iters {
		func(it trie.NodeIterator) {
			g.Go(func() error {
				return s.createSnapshot(ctx, it, headerID, height, seekingPaths)
			})
		}(it)
	}

	return g.Wait()
}

func (s *Service) storageSnapshot(sr common.Hash, headerID string, height *big.Int, statePath []byte, tx Tx) (Tx, error) {
	if bytes.Equal(sr.Bytes(), emptyContractRoot.Bytes()) {
		return tx, nil
	}

	sTrie, err := s.stateDB.OpenTrie(sr)
	if err != nil {
		return nil, err
	}

	it := sTrie.NodeIterator(make([]byte, 0))
	for it.Next(true) {
		res, err := resolveNode(it.Path(), it, s.stateDB.TrieDB())
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
		if err = s.ipfsPublisher.PublishStorageNode(&res.node, headerID, height, statePath, tx); err != nil {
			return nil, err
		}
	}

	return tx, it.Error()
}
