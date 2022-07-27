package snapshot

import (
	"encoding/csv"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/trie"
	log "github.com/sirupsen/logrus"

	iter "github.com/vulcanize/go-eth-state-node-iterator"
)

type trackedIter struct {
	trie.NodeIterator
	tracker *iteratorTracker

	seekedPath []byte // deepest path being seeked from the tracked iterator
}

func (it *trackedIter) Next(descend bool) bool {
	ret := it.NodeIterator.Next(descend)

	// update seeked path
	it.seekedPath = it.seekedPath[:len(it.Path())]
	copy(it.seekedPath, it.Path())

	if !ret {
		if it.tracker.running {
			it.tracker.stopped.Store(it, struct{}{})
		} else {
			log.Errorf("iterator stopped after tracker halted: path=%x", it.Path())
		}
	}
	return ret
}

type iteratorTracker struct {
	recoveryFile string

	started sync.Map
	stopped sync.Map
	running bool
}

func newTracker(file string) iteratorTracker {
	return iteratorTracker{
		recoveryFile: file,
		started:      sync.Map{},
		stopped:      sync.Map{},
		running:      true,
	}
}

func (tr *iteratorTracker) captureSignal() {
	sigChan := make(chan os.Signal, 1)

	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		log.Errorf("Signal received (%v), stopping", sig)
		tr.haltAndDump()
		os.Exit(1)
	}()
}

// Wraps an iterator in a trackedIter. This should not be called once halts are possible.
func (tr *iteratorTracker) tracked(it trie.NodeIterator, recoveredPath []byte) (ret *trackedIter) {
	// create seeked path of max capacity (65) and populate with provided path
	iterSeekedPath := make([]byte, 0, 65)
	if recoveredPath != nil {
		iterSeekedPath = append(iterSeekedPath, recoveredPath...)
	}

	ret = &trackedIter{it, tr, iterSeekedPath}
	tr.started.Store(ret, struct{}{})
	return
}

// dumps iterator path and bounds to a text file so it can be restored later
func (tr *iteratorTracker) dump() error {
	var rows [][]string
	empty := true

	tr.started.Range(func(key, value any) bool {
		empty = false
		it := key.(*trackedIter)

		var endPath []byte
		if impl, ok := it.NodeIterator.(*iter.PrefixBoundIterator); ok {
			endPath = impl.EndPath
		}
		rows = append(rows, []string{
			fmt.Sprintf("%x", it.Path()),
			fmt.Sprintf("%x", endPath),
			fmt.Sprintf("%x", it.seekedPath),
		})

		return true
	})

	if empty {
		// if the tracker state is empty, erase any existing recovery file
		err := os.Remove(tr.recoveryFile)
		if os.IsNotExist(err) {
			err = nil
		}
		return err
	}

	log.Debug("Dumping recovery state to: ", tr.recoveryFile)

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
func (tr *iteratorTracker) restore(tree state.Trie, stateDB state.Database) ([]trie.NodeIterator, error) {
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

		// Force the lower bound path to an even length
		if len(startPath)&0b1 == 1 {
			decrementPath(startPath) // decrement first to avoid skipped nodes
			startPath = append(startPath, 0)
		}

		it := iter.NewPrefixBoundIterator(tree.NodeIterator(iter.HexToKeyBytes(startPath)), endPath)
		ret = append(ret, tr.tracked(it, recoveredPath))
	}
	return ret, nil
}

func (tr *iteratorTracker) haltAndDump() error {
	tr.running = false

	// drain any pending iterators
	tr.stopped.Range(func(key, value any) bool {
		it := key.(*trackedIter)
		tr.started.Delete(it)
		return true
	})

	return tr.dump()
}
