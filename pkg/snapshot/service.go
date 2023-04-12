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
	"github.com/ethereum/go-ethereum/statediff/indexer/ipld"
	"github.com/ethereum/go-ethereum/statediff/indexer/models"
	"github.com/ethereum/go-ethereum/trie"
	iter "github.com/ethereum/go-ethereum/trie/concurrent_iterator"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"

	"github.com/cerc-io/ipld-eth-state-snapshot/pkg/prom"
	. "github.com/cerc-io/ipld-eth-state-snapshot/pkg/types"
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
	kvdb, _ := rawdb.NewLevelDBDatabase(con.LevelDBPath, 1024, 256, "ipld-eth-state-snapshot", true)
	edb, err := rawdb.NewDatabaseWithFreezer(kvdb, con.AncientDBPath, "ipld-eth-state-snapshot", true)
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

	ctx, cancelCtx := context.WithCancel(context.Background())
	s.tracker = newTracker(s.recoveryFile, int(params.Workers))
	s.tracker.captureSignal(cancelCtx)

	var iters []trie.NodeIterator
	// attempt to restore from recovery file if it exists
	iters, err = s.tracker.restore(tree)
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
	} else {
		// nothing to restore
		log.Debugf("no iterators to restore")
		if params.Workers > 1 {
			iters = iter.SubtrieIterators(tree, params.Workers)
		} else {
			iters = []trie.NodeIterator{tree.NodeIterator(nil)}
		}
		for i, it := range iters {
			// recovered path is nil for fresh iterators
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
		return s.createSnapshotAsync(ctx, iters, headerID, new(big.Int).SetUint64(params.Height), paths)
	case len(iters) == 1:
		return s.createSnapshot(ctx, iters[0], headerID, new(big.Int).SetUint64(params.Height), paths)
	default:
		return nil
	}
}

// CreateLatestSnapshot snapshot at head (ignores height param)
func (s *Service) CreateLatestSnapshot(workers uint, watchedAddresses map[common.Address]struct{}) error {
	log.Info("Creating snapshot at head")
	hash := rawdb.ReadHeadHeaderHash(s.ethDB)
	height := rawdb.ReadHeaderNumber(s.ethDB, hash)
	if height == nil {
		return fmt.Errorf("unable to read header height for header hash %s", hash.String())
	}
	return s.CreateSnapshot(SnapshotParams{Height: *height, Workers: workers, WatchedAddresses: watchedAddresses})
}

// Full-trie concurrent snapshot
func (s *Service) createSnapshotAsync(ctx context.Context, iters []trie.NodeIterator, headerID string, height *big.Int, seekingPaths [][]byte) error {
	// use errgroup with a context to stop all concurrent iterators if one runs into an error
	// each concurrent iterator completes processing it's current node before stopping
	g, ctx := errgroup.WithContext(ctx)
	for _, it := range iters {
		func(it trie.NodeIterator) {
			g.Go(func() error {
				return s.createSnapshot(ctx, it, headerID, height, seekingPaths)
			})
		}(it)
	}

	return g.Wait()
}

// createSnapshot performs traversal using the given iterator and indexes the nodes
// optionally filtering them according to a list of paths
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

	// path (from recovery dump) to be seeked on recovery
	// nil in case of a fresh iterator
	var recoveredPath []byte

	// latest path seeked from the concurrent iterator
	// (updated after a node processed)
	// nil in case of a fresh iterator; initially holds the recovered path in case of a recovered iterator
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

// createSubTrieSnapshot processes nodes at the next level of a trie using the given subtrie iterator
// continually updating seekedPath with path of the latest processed node
func (s *Service) createSubTrieSnapshot(ctx context.Context, tx Tx, prefixPath []byte, subTrieIt trie.NodeIterator,
	recoveredPath []byte, seekedPath *[]byte, endPath []byte, headerID string, height *big.Int, seekingPaths [][]byte) error {
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

			// move on to next node if current path is empty
			// occurs when reaching root node or just before reaching the first child of a subtrie in case of some concurrent iterators
			if bytes.Equal(subTrieIt.Path(), []byte{}) {
				// if node path is empty and prefix is nil, it's the root node
				if prefixPath == nil {
					// create snapshot of node, if it is a leaf this will also create snapshot of entire storage trie
					if err := s.createNodeSnapshot(tx, subTrieIt, headerID, height, seekingPaths); err != nil {
						return err
					}
					updateSeekedPath(seekedPath, subTrieIt.Path())
				}

				if ok := subTrieIt.Next(true); !ok {
					// return if no further nodes available
					return subTrieIt.Error()
				}
			}

			// create the full node path as it.Path() doesn't include the path before subtrie root
			nodePath := append(prefixPath, subTrieIt.Path()...)

			// check iterator upper bound before processing the node
			// required to avoid processing duplicate nodes:
			//   if a node is considered more than once,
			//   it's whole subtrie is re-processed giving large number of duplicate nodoes
			if !checkUpperPathBound(nodePath, endPath) {
				// fmt.Println("failed checkUpperPathBound", nodePath, endPath)
				// explicitly stop the iterator in tracker if upper bound check fails
				// required since it won't be marked as stopped if further nodes are still available
				if trackedSubtrieIt, ok := subTrieIt.(*trackedIter); ok {
					s.tracker.stopIter(trackedSubtrieIt)
				}
				return subTrieIt.Error()
			}

			// skip the current node if it's before recovered path and not along the recovered path
			// nodes at the same level that are before recovered path are ignored to avoid duplicate nodes
			// however, nodes along the recovered path are re-considered for redundancy
			if bytes.Compare(recoveredPath, nodePath) > 0 &&
				// a node is along the recovered path if it's path is shorter or equal in length
				// and is part of the recovered path
				!(len(nodePath) <= len(recoveredPath) && bytes.Equal(recoveredPath[:len(nodePath)], nodePath)) {
				continue
			}

			// ignore node if it is not along paths of interest
			if s.watchingAddresses && !validPath(nodePath, seekingPaths) {
				// consider this node as processed since it is getting ignored
				// and update the seeked path
				updateSeekedPath(seekedPath, nodePath)
				// move on to the next node
				continue
			}

			// if the node is along paths of interest
			// create snapshot of node, if it is a leaf this will also create snapshot of entire storage trie
			if err := s.createNodeSnapshot(tx, subTrieIt, headerID, height, seekingPaths); err != nil {
				return err
			}
			// update seeked path after node has been processed
			updateSeekedPath(seekedPath, nodePath)

			// create an iterator to traverse and process the next level of this subTrie
			nextSubTrieIt, err := s.createSubTrieIt(nodePath, subTrieIt.Hash(), recoveredPath)
			if err != nil {
				return err
			}
			// pass on the seekedPath of the tracked concurrent iterator to be updated
			if err := s.createSubTrieSnapshot(ctx, tx, nodePath, nextSubTrieIt, recoveredPath, seekedPath, endPath, headerID, height, seekingPaths); err != nil {
				return err
			}
		}
	}
}

// createSubTrieIt creates an iterator to traverse the subtrie of node with the given hash
// the subtrie iterator is initialized at a node from the recovered path at corresponding level (if avaiable)
func (s *Service) createSubTrieIt(prefixPath []byte, hash common.Hash, recoveredPath []byte) (trie.NodeIterator, error) {
	// skip directly to the node from the recovered path at corresponding level
	// applicable if:
	//   node path is behind recovered path
	//   and recovered path includes the prefix path
	var startPath []byte
	if bytes.Compare(recoveredPath, prefixPath) > 0 &&
		len(recoveredPath) > len(prefixPath) &&
		bytes.Equal(recoveredPath[:len(prefixPath)], prefixPath) {
		startPath = append(startPath, recoveredPath[len(prefixPath):len(prefixPath)+1]...)
		// force the lower bound path to an even length
		// (required by HexToKeyBytes())
		if len(startPath)&0b1 == 1 {
			// decrement first to avoid skipped nodes
			decrementPath(startPath)
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

// createNodeSnapshot indexes the current node
// entire storage trie is also indexed (if available)
func (s *Service) createNodeSnapshot(tx Tx, it trie.NodeIterator, headerID string, height *big.Int, watchedAddressesLeafPaths [][]byte) error {
	tx, err := s.ipfsPublisher.PrepareTxForBatch(tx, s.maxBatchSize)
	if err != nil {
		return err
	}

	// index values by leaf key
	if it.Leaf() {
		// if it is a "value" node, we will index the value by leaf key
		// publish codehash => code mappings
		// take storage snapshot
		if err := s.processStateValueNode(it, headerID, height, watchedAddressesLeafPaths, tx); err != nil {
			return err
		}
	} else { // trie nodes will be written to blockstore only
		// reminder that this includes leaf nodes, since the geth iterator.Leaf() actually signifies a "value" node
		// so this is also where we publish the IPLD block corresponding to the "value" nodes indexed above
		if IsNullHash(it.Hash()) {
			// skip null node
			return nil
		}
		nodeVal := make([]byte, len(it.NodeBlob()))
		copy(nodeVal, it.NodeBlob())
		if len(watchedAddressesLeafPaths) > 0 {
			var elements []interface{}
			if err := rlp.DecodeBytes(nodeVal, &elements); err != nil {
				return err
			}
			ok, err := isLeaf(elements)
			if err != nil {
				return err
			}
			if ok {
				nodePath := make([]byte, len(it.Path()))
				copy(nodePath, it.Path())
				partialPath := trie.CompactToHex(elements[0].([]byte))
				valueNodePath := append(nodePath, partialPath...)
				if !isWatchedAddress(watchedAddressesLeafPaths, valueNodePath) {
					// skip this node
					return nil
				}
			}
		}
		nodeHash := make([]byte, len(it.Hash().Bytes()))
		copy(nodeHash, it.Hash().Bytes())
		if _, err := s.ipfsPublisher.PublishIPLD(ipld.Keccak256ToCid(ipld.MEthStateTrie, nodeHash), nodeVal, height, tx); err != nil {
			return err
		}
	}

	return it.Error()
}

// reminder: it.Leaf() == true when the iterator is positioned at a "value node" which is not something that actually exists in an MMPT
func (s *Service) processStateValueNode(it trie.NodeIterator, headerID string, height *big.Int,
	watchedAddressesLeafPaths [][]byte, tx Tx) error {
	// skip if it is not a watched address
	// If we aren't watching any specific addresses, we are watching everything
	if len(watchedAddressesLeafPaths) > 0 && !isWatchedAddress(watchedAddressesLeafPaths, it.Path()) {
		return nil
	}

	// created vs updated is important for leaf nodes since we need to diff their storage
	// so we need to map all changed accounts at B to their leafkey, since account can change pathes but not leafkey
	var account types.StateAccount
	accountRLP := make([]byte, len(it.LeafBlob()))
	copy(accountRLP, it.LeafBlob())
	if err := rlp.DecodeBytes(accountRLP, &account); err != nil {
		return fmt.Errorf("error decoding account for leaf value at leaf key %x\nerror: %v", it.LeafKey(), err)
	}
	leafKey := make([]byte, len(it.LeafKey()))
	copy(leafKey, it.LeafKey())

	// write codehash => code mappings if we have a contract
	if !bytes.Equal(account.CodeHash, emptyCodeHash) {
		codeHash := common.BytesToHash(account.CodeHash)
		code, err := s.stateDB.ContractCode(common.Hash{}, codeHash)
		if err != nil {
			return fmt.Errorf("failed to retrieve code for codehash %s\r\n error: %v", codeHash.String(), err)
		}
		if _, err := s.ipfsPublisher.PublishIPLD(ipld.Keccak256ToCid(ipld.RawBinary, codeHash.Bytes()), code, height, tx); err != nil {
			return err
		}
	}

	// since this is a "value node", we need to move up to the "parent" node which is the actual leaf node
	// it should be in the fastcache since it necessarily was recently accessed to reach the current node
	parentNodeRLP, err := s.stateDB.TrieDB().Node(it.Parent())
	if err != nil {
		return err
	}
	// publish the state leaf model
	stateKeyStr := common.BytesToHash(leafKey).String()
	stateLeafNodeModel := &models.StateNodeModel{
		BlockNumber: height.String(),
		HeaderID:    headerID,
		StateKey:    stateKeyStr,
		Removed:     false,
		CID:         ipld.Keccak256ToCid(ipld.MEthStateTrie, crypto.Keccak256(parentNodeRLP)).String(),
		Diff:        false,
		Balance:     account.Balance.String(),
		Nonce:       account.Nonce,
		CodeHash:    common.BytesToHash(account.CodeHash).String(),
		StorageRoot: account.Root.String(),
	}
	if err := s.ipfsPublisher.PublishStateLeafNode(stateLeafNodeModel, tx); err != nil {
		return fmt.Errorf("failed publishing state leaf node for leaf key %s\r\nerror: %w", stateKeyStr, err)
	}
	// create storage snapshot
	// this short circuits if storage is empty
	if _, err := s.storageSnapshot(account.Root, stateKeyStr, headerID, height, tx); err != nil {
		return fmt.Errorf("failed building storage snapshot for account %+v\r\nerror: %w", account, err)
	}
	return nil
}

func (s *Service) storageSnapshot(sr common.Hash, stateKey, headerID string, height *big.Int, tx Tx) (Tx, error) {
	if bytes.Equal(sr.Bytes(), emptyContractRoot.Bytes()) {
		return tx, nil
	}

	sTrie, err := s.stateDB.OpenTrie(sr)
	if err != nil {
		return nil, err
	}

	it := sTrie.NodeIterator(make([]byte, 0))
	for it.Next(true) {
		if it.Leaf() {
			if err := s.processStorageValueNode(it, stateKey, headerID, height, tx); err != nil {
				return nil, err
			}
		} else {
			nodeVal := make([]byte, len(it.NodeBlob()))
			copy(nodeVal, it.NodeBlob())
			nodeHash := make([]byte, len(it.Hash().Bytes()))
			copy(nodeHash, it.Hash().Bytes())
			if _, err := s.ipfsPublisher.PublishIPLD(ipld.Keccak256ToCid(ipld.MEthStorageTrie, nodeHash), nodeVal, height, tx); err != nil {
				return nil, err
			}
		}
	}

	return tx, it.Error()
}

// reminder: it.Leaf() == true when the iterator is positioned at a "value node" which is not something that actually exists in an MMPT
func (s *Service) processStorageValueNode(it trie.NodeIterator, stateKey, headerID string, height *big.Int, tx Tx) error {
	// skip if it is not a watched address
	leafKey := make([]byte, len(it.LeafKey()))
	copy(leafKey, it.LeafKey())
	value := make([]byte, len(it.LeafBlob()))
	copy(value, it.LeafBlob())

	// since this is a "value node", we need to move up to the "parent" node which is the actual leaf node
	// it should be in the fastcache since it necessarily was recently accessed to reach the current node
	parentNodeRLP, err := s.stateDB.TrieDB().Node(it.Parent())
	if err != nil {
		return err
	}

	// publish storage leaf node model
	storageLeafKeyStr := common.BytesToHash(leafKey).String()
	storageLeafNodeModel := &models.StorageNodeModel{
		BlockNumber: height.String(),
		HeaderID:    headerID,
		StateKey:    stateKey,
		StorageKey:  storageLeafKeyStr,
		Removed:     false,
		CID:         ipld.Keccak256ToCid(ipld.MEthStorageTrie, crypto.Keccak256(parentNodeRLP)).String(),
		Diff:        false,
		Value:       value,
	}
	if err := s.ipfsPublisher.PublishStorageLeafNode(storageLeafNodeModel, tx); err != nil {
		return fmt.Errorf("failed to publish storage leaf node for state leaf key %s and storage leaf key %s\r\nerr: %w", stateKey, storageLeafKeyStr, err)
	}
	return nil
}

// validPath checks if a path is prefix to any one of the paths in the given list
func validPath(currentPath []byte, seekingPaths [][]byte) bool {
	for _, seekingPath := range seekingPaths {
		if bytes.HasPrefix(seekingPath, currentPath) {
			return true
		}
	}
	return false
}

// isWatchedAddress is used to check if a state account corresponds to one of the addresses the builder is configured to watch
func isWatchedAddress(watchedAddressesLeafPaths [][]byte, valueNodePath []byte) bool {
	for _, watchedAddressPath := range watchedAddressesLeafPaths {
		if bytes.Equal(watchedAddressPath, valueNodePath) {
			return true
		}
	}

	return false
}

// isLeaf checks if the node we are at is a leaf
func isLeaf(elements []interface{}) (bool, error) {
	if len(elements) > 2 {
		return false, nil
	}
	if len(elements) < 2 {
		return false, fmt.Errorf("node cannot be less than two elements in length")
	}
	switch elements[0].([]byte)[0] / 16 {
	case '\x00':
		return false, nil
	case '\x01':
		return false, nil
	case '\x02':
		return true, nil
	case '\x03':
		return true, nil
	default:
		return false, fmt.Errorf("unknown hex prefix")
	}
}
