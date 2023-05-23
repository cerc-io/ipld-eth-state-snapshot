package snapshot

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/cerc-io/ipld-eth-state-snapshot/pkg/prom"

	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/trie"
	iter "github.com/ethereum/go-ethereum/trie/concurrent_iterator"
	log "github.com/sirupsen/logrus"
)

var trackedIterCount int32

type trackedIter struct {
	id int32
	mu sync.Mutex
	trie.NodeIterator
	tracker *iteratorTracker

	seekedPath []byte // latest full node path seeked from the tracked iterator
	startPath  []byte // startPath for the tracked iterator
	endPath    []byte // endPath for the tracked iterator
	lastPath   []byte // latest it.Path() (not the full node path) seeked
}

func (it *trackedIter) getLastPath() []byte {
	it.mu.Lock()
	defer it.mu.Unlock()

	return it.lastPath
}

func (it *trackedIter) setLastPath(val []byte) {
	it.mu.Lock()
	defer it.mu.Unlock()

	it.lastPath = val
}

func (it *trackedIter) Next(descend bool) bool {
	ret := it.NodeIterator.Next(descend)

	if !ret {
		if it.tracker.running {
			it.tracker.stopChan <- it
		} else {
			log.Errorf("iterator stopped after tracker halted: path=%x", it.Path())
		}
		it.setLastPath(it.endPath)
	} else {
		it.setLastPath(it.Path())
	}
	return ret
}

type iteratorTracker struct {
	recoveryFile string

	startChan chan *trackedIter
	stopChan  chan *trackedIter
	started   map[*trackedIter]struct{}
	stopped   []*trackedIter
	running   bool
}

func newTracker(file string, buf int) iteratorTracker {
	return iteratorTracker{
		recoveryFile: file,
		startChan:    make(chan *trackedIter, buf),
		stopChan:     make(chan *trackedIter, buf),
		started:      map[*trackedIter]struct{}{},
		running:      true,
	}
}

func (tr *iteratorTracker) captureSignal(cancelCtx context.CancelFunc) {
	sigChan := make(chan os.Signal, 1)

	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		log.Errorf("Signal received (%v), stopping", sig)
		// cancel context on receiving a signal
		// on ctx cancellation, all the iterators complete processing of their current node before stopping
		cancelCtx()
	}()
}

// Wraps an iterator in a trackedIter. This should not be called once halts are possible.
func (tr *iteratorTracker) tracked(it trie.NodeIterator, recoveredPath []byte) (ret *trackedIter) {
	// create seeked path of max capacity (65)
	iterSeekedPath := make([]byte, 0, 65)
	// intially populate seeked path with the recovered path
	// to be used in trie traversal
	if recoveredPath != nil {
		iterSeekedPath = append(iterSeekedPath, recoveredPath...)
	}

	// if the iterator being tracked is a PrefixBoundIterator, capture it's end path
	// to be used in trie traversal
	var endPath []byte
	var startPath []byte
	if boundedIter, ok := it.(*iter.PrefixBoundIterator); ok {
		startPath = boundedIter.StartPath
		endPath = boundedIter.EndPath
	}

	ret = &trackedIter{
		atomic.AddInt32(&trackedIterCount, 1),
		sync.Mutex{},
		it,
		tr,
		iterSeekedPath,
		startPath,
		endPath,
		nil,
	}
	tr.startChan <- ret

	if prom.Enabled() {
		pathDepth := max(max(len(startPath), len(endPath)), 1)
		totalSteps := estimateSteps(startPath, endPath, pathDepth)
		prom.RegisterGaugeFunc(
			fmt.Sprintf("tracked_iterator_%d", ret.id),
			func() float64 {
				lastPath := ret.getLastPath()
				if nil == lastPath {
					return 0.0
				}
				remainingSteps := estimateSteps(lastPath, endPath, pathDepth)
				if remainingSteps > 0 {
					return (float64(totalSteps) - float64(remainingSteps)) / float64(totalSteps) * 100.0
				} else {
					return 100.0
				}
			})
	}

	return
}

// explicitly stops an iterator
func (tr *iteratorTracker) stopIter(it *trackedIter) {
	tr.stopChan <- it
}

// dumps iterator path and bounds to a text file so it can be restored later
func (tr *iteratorTracker) dump() error {
	log.Debug("Dumping recovery state to: ", tr.recoveryFile)
	var rows [][]string
	for it := range tr.started {
		var startPath []byte
		var endPath []byte
		if impl, ok := it.NodeIterator.(*iter.PrefixBoundIterator); ok {
			// if the iterator being tracked is a PrefixBoundIterator,
			// initialize start and end paths with its bounds
			startPath = impl.StartPath
			endPath = impl.EndPath
		}

		// if seeked path and iterator path are non-empty, use iterator's path as startpath
		if !bytes.Equal(it.seekedPath, []byte{}) && !bytes.Equal(it.Path(), []byte{}) {
			startPath = it.Path()
		}

		rows = append(rows, []string{
			fmt.Sprintf("%x", startPath),
			fmt.Sprintf("%x", endPath),
			fmt.Sprintf("%x", it.seekedPath),
		})
	}

	file, err := os.Create(tr.recoveryFile)
	if err != nil {
		return err
	}
	defer file.Close()
	out := csv.NewWriter(file)

	return out.WriteAll(rows)
}

// attempts to read iterator state from file
// if file doesn't exist, returns an empty slice with no error
func (tr *iteratorTracker) restore(tree state.Trie) ([]trie.NodeIterator, error) {
	file, err := os.Open(tr.recoveryFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	log.Debug("Restoring recovery state from: ", tr.recoveryFile)
	defer file.Close()
	in := csv.NewReader(file)
	in.FieldsPerRecord = 3
	rows, err := in.ReadAll()
	if err != nil {
		return nil, err
	}

	var ret []trie.NodeIterator
	for _, row := range rows {
		// pick up where each interval left off
		var startPath []byte
		var endPath []byte
		var recoveredPath []byte

		if len(row[0]) != 0 {
			if _, err = fmt.Sscanf(row[0], "%x", &startPath); err != nil {
				return nil, err
			}
		}
		if len(row[1]) != 0 {
			if _, err = fmt.Sscanf(row[1], "%x", &endPath); err != nil {
				return nil, err
			}
		}
		if len(row[2]) != 0 {
			if _, err = fmt.Sscanf(row[2], "%x", &recoveredPath); err != nil {
				return nil, err
			}
		}

		// force the lower bound path to an even length
		// (required by HexToKeyBytes())
		if len(startPath)&0b1 == 1 {
			// decrement first to avoid skipped nodes
			decrementPath(startPath)
			startPath = append(startPath, 0)
		}

		it := iter.NewPrefixBoundIterator(tree.NodeIterator(iter.HexToKeyBytes(startPath)), startPath, endPath)
		ret = append(ret, tr.tracked(it, recoveredPath))
	}
	return ret, nil
}

func (tr *iteratorTracker) haltAndDump() error {
	tr.running = false

	// drain any pending iterators
	close(tr.startChan)
	for start := range tr.startChan {
		tr.started[start] = struct{}{}
	}
	close(tr.stopChan)
	for stop := range tr.stopChan {
		tr.stopped = append(tr.stopped, stop)
	}

	for _, stop := range tr.stopped {
		delete(tr.started, stop)
	}

	if len(tr.started) == 0 {
		// if the tracker state is empty, erase any existing recovery file
		err := os.Remove(tr.recoveryFile)
		if os.IsNotExist(err) {
			err = nil
		}
		return err
	}

	return tr.dump()
}
