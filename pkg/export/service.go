// Copyright © 2022 Vulcanize, Inc
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

package export

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/ethereum/go-ethereum/params"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"
	log "github.com/sirupsen/logrus"

	iter "github.com/cerc-io/go-eth-state-node-iterator"
	"github.com/cerc-io/ipld-eth-state-snapshot/pkg/prom"
	"github.com/cerc-io/ipld-eth-state-snapshot/pkg/shared"
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
	exportDB, importDB ethdb.Database
	exportStateDB      state.Database
	maxBatchSize       uint
	tracker            shared.IteratorTracker
	recoveryFile       string
}

// OpenLevelDBs opens read and write leveldbs
func OpenLevelDBs(con *Config) (ethdb.Database, ethdb.Database, error) {
	exportDB, err := rawdb.NewLevelDBDatabaseWithFreezer(
		con.ExportLevelDBPath, 1024, 256, con.ExportAncientDBPath, "export", true,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to open export db: %s", err)
	}
	importDB, err := rawdb.NewLevelDBDatabaseWithFreezer(
		con.ImportLevelDBPath, 1024, 256, con.ImportAncientDBPath, "import", true,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to open import db: %s", err)
	}
	return exportDB, importDB, nil
}

// NewExportService creates &export.Service.
func NewExportService(exportDB, importDB ethdb.Database, recoveryFile string) (*Service, error) {
	return &Service{
		exportDB:      exportDB,
		importDB:      importDB,
		exportStateDB: state.NewDatabase(exportDB),
		maxBatchSize:  defaultBatchSize,
		recoveryFile:  recoveryFile,
	}, nil
}

type Params struct {
	Height      uint64
	TrieWorkers uint
	SegmentSize uint64
}

// Export exports the state and block data required to start a full sync at a specific block height
func (s *Service) Export(ctx context.Context, wg *sync.WaitGroup, p Params) <-chan error {
	errChan := make(chan error)
	s.exportStateAndStorage(ctx, wg, p, errChan)
	s.exportBlockData(ctx, wg, p, errChan)
	return errChan
}

func (s *Service) exportBlockData(ctx context.Context, wg *sync.WaitGroup, p Params, errChan chan<- error) {
	// ancient data has to be processed in order
	wg.Add(2)
	// Determine which blocks should go into ancient file vs leveldb storage
	// the data that needs to end up in the new leveldb may in the exportDB's ancient/freezerdb
	ancientSegments, nascentSegments := splitAndSegmentRange(p.SegmentSize, 0, p.Height)
	go func() {
		defer wg.Done()
		for _, rng := range ancientSegments {
			log.Debugf("processing ancient block segment (%d, %d)", rng[0], rng[1])
			if err := s.exportBlocksReceiptsTDAndHashes(true, rng[0], rng[1]); err != nil {
				errChan <- fmt.Errorf("error processing ancient range: (%d, %d); err: %s", rng[0], rng[1], err.Error())
				return
			}
			select {
			case <-ctx.Done():
				log.Infof("quit signal received, stopping ancient block processing. Last range processed: (%d, %d)", rng[0], rng[1])
				return
			default:
			}
		}
		log.Info("finished processing ancient block segments")
	}()
	// for simplicity and since there isn't much nascent block data, we process it in order too
	go func() {
		defer wg.Done()
		for _, rng := range nascentSegments {
			log.Debugf("processing nascent block segment (%d, %d)", rng[0], rng[1])
			if err := s.exportBlocksReceiptsTDAndHashes(false, rng[0], rng[1]); err != nil {
				errChan <- fmt.Errorf("error processing nascent range: (%d, %d); err: %s", rng[0], rng[1], err.Error())
				return
			}
			select {
			case <-ctx.Done():
				log.Infof("quit signal received, stopping nascent block processing. Last range processed: (%d, %d)", rng[0], rng[1])
				return
			default:
			}
		}
		log.Info("finished processing nascent block segments")
	}()
}

func (s *Service) exportBlocksReceiptsTDAndHashes(ancient bool, start, stop uint64) error {
	limit := int(stop - start + 1)
	heights, hashes := rawdb.ReadAllCanonicalHashes(s.exportDB, start, stop, limit)
	if len(hashes) != limit {
		return fmt.Errorf("number of read canonical (%d) hashes does not match expected number (%d)", len(hashes), limit)
	}
	if len(heights) != len(hashes) {
		return fmt.Errorf("number of heights (%d) does not match number of hashes (%d)", heights, hashes)
	}
	blocks := make(types.Blocks, len(heights))
	receiptsList := make([]types.Receipts, len(heights))
	for i, hash := range hashes {
		height := heights[i]
		block := rawdb.ReadBlock(s.exportDB, hash, height)
		if block == nil {
			return fmt.Errorf("nil block found for height %d hash %s", height, hash.Hex())
		}
		receipts := rawdb.ReadRawReceipts(s.exportDB, hash, height)
		td := rawdb.ReadTd(s.exportDB, hash, height)
		if td == nil {
			return fmt.Errorf("nil total difficulty found for height %d hash %s", height, hash.Hex())
		}
		blocks[i] = block
		receiptsList[i] = receipts
		if ancient {
			if _, err := rawdb.WriteAncientBlocks(s.importDB, []*types.Block{block}, []types.Receipts{receipts}, td); err != nil {
				return fmt.Errorf("unable to write ancient block data to importDB at height %d hash %s err %s", height, hash.Hex(), err.Error())
			}
		} else {
			rawdb.WriteReceipts(s.importDB, hash, height, receipts)
			rawdb.WriteBlock(s.importDB, block)
			rawdb.WriteTd(s.importDB, hash, height, td)
			rawdb.WriteCanonicalHash(s.importDB, hash, height)
		}
	}
	return nil
}

// height - params.FullImmutabilityThreshold - 1
func (s *Service) exportStateAndStorage(ctx context.Context, wg *sync.WaitGroup, p Params, errChan chan<- error) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		hash := rawdb.ReadCanonicalHash(s.exportDB, p.Height)
		header := rawdb.ReadHeader(s.exportDB, hash, p.Height)
		if header == nil {
			errChan <- fmt.Errorf("unable to read canonical header at height %d", p.Height)
			return
		}

		tree, err := s.exportStateDB.OpenTrie(header.Root)
		if err != nil {
			errChan <- fmt.Errorf("unable to open trie for root %s", header.Root.Hex())
			return
		}
		s.tracker = shared.NewTracker(s.recoveryFile, int(p.TrieWorkers))

		var iters []trie.NodeIterator
		// attempt to restore from recovery file if it exists
		iters, err = s.tracker.Restore(tree)
		if err != nil {
			errChan <- fmt.Errorf("restore error: %s", err.Error())
			return
		}

		if iters != nil {
			log.Debugf("restored iterators; count: %d", len(iters))
			if p.TrieWorkers < uint(len(iters)) {
				errChan <- fmt.Errorf(
					"number of recovered workers (%d) is greater than number configured (%d)",
					len(iters), p.TrieWorkers,
				)
				return
			}
		} else {
			// nothing to restore
			log.Debugf("no iterators to restore")
			if p.TrieWorkers > 1 {
				iters = iter.SubtrieIterators(tree, p.TrieWorkers)
			} else {
				iters = []trie.NodeIterator{tree.NodeIterator(nil)}
			}
			for i, it := range iters {
				// recovered path is nil for fresh iterators
				iters[i] = s.tracker.Tracked(it, nil)
			}
		}

		defer func() {
			err := s.tracker.HaltAndDump()
			if err != nil {
				log.Errorf("failed to write recovery file: %v", err)
			}
		}()

		switch {
		case len(iters) > 1:
			s.createSnapshotAsync(ctx, iters, errChan)
		case len(iters) == 1:
			if err := s.createSnapshot(ctx, iters[0]); err != nil {
				errChan <- err
			}
		default:
			errChan <- fmt.Errorf("number of workers (%d) needs to be greater than 0", len(iters))
			return
		}
	}()
}

// Full-trie concurrent snapshot
func (s *Service) createSnapshotAsync(ctx context.Context, iters []trie.NodeIterator, errChan chan<- error) {
	wg := new(sync.WaitGroup)
	for _, it := range iters {
		wg.Add(1)
		go func(it trie.NodeIterator) {
			defer wg.Done()
			if err := s.createSnapshot(ctx, it); err != nil {
				errChan <- err
			}
		}(it)
	}
	wg.Wait()
}

// createSnapshot performs traversal using the given iterator and indexes the nodes
// optionally filtering them according to a list of paths
func (s *Service) createSnapshot(ctx context.Context, it trie.NodeIterator) error {
	// path (from recovery dump) to be seeked on recovery
	// nil in case of a fresh iterator
	var recoveredPath []byte

	// latest path seeked from the concurrent iterator
	// (updated after a node processed)
	// nil in case of a fresh iterator; initially holds the recovered path in case of a recovered iterator
	var seekedPath *[]byte

	// end path for the concurrent iterator
	var endPath []byte

	if it, ok := it.(*shared.TrackedIter); ok {
		seekedPath = &it.SeekedPath
		recoveredPath = append(recoveredPath, *seekedPath...)
		endPath = it.EndPath
	} else {
		return errors.New("untracked iterator")
	}

	return s.createSubTrieSnapshot(ctx, nil, it, recoveredPath, seekedPath, endPath)
}

// createSubTrieSnapshot processes nodes at the next level of a trie using the given subtrie iterator
// continually updating seekedPath with path of the latest processed node
func (s *Service) createSubTrieSnapshot(ctx context.Context, prefixPath []byte, subTrieIt trie.NodeIterator,
	recoveredPath []byte, seekedPath *[]byte, endPath []byte) error {
	prom.IncActiveIterCount()
	defer prom.DecActiveIterCount()
	// descend in the first loop iteration to reach first child node
	descend := true
	for {
		select {
		case <-ctx.Done():
			log.Info("quit signal received, canceling subtrie snapshotting")
			return nil
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
					if err := s.createNodeSnapshot(subTrieIt.Path(), subTrieIt); err != nil {
						return fmt.Errorf("unable to create node snapshot: %s", err.Error())
					}
					shared.UpdateSeekedPath(seekedPath, subTrieIt.Path())
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
			//   it's whole subtrie is re-processed giving large number of duplicate nodes
			if !shared.CheckUpperPathBound(nodePath, endPath) {
				// explicitly stop the iterator in tracker if upper bound check fails
				// required since it won't be marked as stopped if further nodes are still available
				if trackedSubtrieIt, ok := subTrieIt.(*shared.TrackedIter); ok {
					s.tracker.StopIt(trackedSubtrieIt)
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

			// create snapshot of node, if it is a leaf this will also create snapshot of entire storage trie
			if err := s.createNodeSnapshot(nodePath, subTrieIt); err != nil {
				return err
			}
			// update seeked path after node has been processed
			shared.UpdateSeekedPath(seekedPath, nodePath)

			// create an iterator to traverse and process the next level of this subTrie
			nextSubTrieIt, err := s.createSubTrieIt(nodePath, subTrieIt.Hash(), recoveredPath)
			if err != nil {
				return err
			}
			// pass on the seekedPath of the tracked concurrent iterator to be updated
			return s.createSubTrieSnapshot(ctx, nodePath, nextSubTrieIt, recoveredPath, seekedPath, endPath)
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
			shared.DecrementPath(startPath)
			startPath = append(startPath, 0)
		}
	}

	// create subTrie iterator with the given hash
	subTrie, err := s.exportStateDB.OpenTrie(hash)
	if err != nil {
		return nil, err
	}

	return subTrie.NodeIterator(iter.HexToKeyBytes(startPath)), nil
}

// createNodeSnapshot indexes the current node
// entire storage trie is also indexed (if available)
func (s *Service) createNodeSnapshot(path []byte, it trie.NodeIterator) error {
	res, err := exportNode(s.importDB, path, it, s.exportStateDB.TrieDB())
	if err != nil {
		return err
	}
	if res == nil {
		return nil
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

		// publish any non-nil code referenced by codehash
		if !bytes.Equal(account.CodeHash, emptyCodeHash) {
			codeHash := common.BytesToHash(account.CodeHash)
			codeBytes := rawdb.ReadCode(s.exportDB, codeHash)
			if len(codeBytes) == 0 {
				log.Error("Code is missing", "account", common.BytesToHash(it.LeafKey()))
				return errors.New("missing code")
			}
			rawdb.WriteCode(s.importDB, codeHash, codeBytes)
		}

		if err := s.storageSnapshot(account.Root, res.node.Path); err != nil {
			return fmt.Errorf("failed building storage snapshot for account %+v\r\nerror: %w", account, err)
		}
	case Extension, Branch:
		// nothing else to do for non-leaf nodes, the raw node was already written in exportNode()
	default:
		return errors.New("unexpected node type")
	}
	return it.Error()
}

func (s *Service) storageSnapshot(sr common.Hash, statePath []byte) error {
	if bytes.Equal(sr.Bytes(), emptyContractRoot.Bytes()) {
		return nil
	}

	sTrie, err := s.exportStateDB.OpenTrie(sr)
	if err != nil {
		return err
	}

	it := sTrie.NodeIterator(make([]byte, 0))
	for it.Next(true) {
		if it.Leaf() {
			return nil
		}
		if IsNullHash(it.Hash()) {
			return nil
		}

		n, err := s.exportStateDB.TrieDB().Node(it.Hash())
		if err != nil {
			return err
		}
		// write the raw node to the importDB
		rawdb.WriteTrieNode(s.importDB, it.Hash(), n)
	}

	return it.Error()
}

// store in ancients if <= height - params.FullImmutabilityThreshold - 1
func splitAndSegmentRange(size, start, stop uint64) ([][2]uint64, [][2]uint64) {
	if stop >= params.FullImmutabilityThreshold+1 {
		ancientStop := stop - params.FullImmutabilityThreshold - 1
		ancientStart := start
		start = ancientStop + 1
		return segmentRange(size, ancientStart, ancientStop), segmentRange(size, start, stop)
	}
	return nil, segmentRange(size, start, stop)
}

func segmentRange(size, start, stop uint64) [][2]uint64 {
	numOfSegments := ((stop - start) + 1) / size
	remainder := ((stop - start) + 1) % size
	if remainder > 0 {
		numOfSegments++
	}
	segments := make([][2]uint64, numOfSegments)
	for i := range segments {
		end := start + size - 1
		segments[i] = [2]uint64{start, end}
		start = end + 1
	}
	return segments
}

type nodeResult struct {
	node     Node
	elements []interface{}
}

func exportNode(importDB ethdb.Database, nodePath []byte, it trie.NodeIterator, exportTrieDB *trie.Database) (*nodeResult, error) {
	// "leaf" nodes are actually "value" nodes, whose parents are the actual leaves
	if it.Leaf() {
		return nil, nil
	}
	if IsNullHash(it.Hash()) {
		return nil, nil
	}

	// use full node path
	// (it.Path() will give partial path in case of subtrie iterators)
	path := make([]byte, len(nodePath))
	copy(path, nodePath)
	n, err := exportTrieDB.Node(it.Hash())
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
	// write the raw node to the importDB
	rawdb.WriteTrieNode(importDB, it.Hash(), n)

	return &nodeResult{
		node: Node{
			NodeType: ty,
			Path:     path,
			Value:    n,
		},
		elements: elements,
	}, nil
}
