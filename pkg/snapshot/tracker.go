package snapshot

import (
	"encoding/csv"
	"fmt"
	"os"

	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/trie"

	iter "github.com/vulcanize/go-eth-state-node-iterator"
)

type trackedIter struct {
	trie.NodeIterator
	tracker *iteratorTracker
}

func (it *trackedIter) Next(descend bool) bool {
	ret := it.NodeIterator.Next(descend)
	if !ret {
		it.tracker.stopChan <- it
	}
	return ret
}

type iteratorTracker struct {
	startChan chan *trackedIter
	stopChan  chan *trackedIter
	started   map[*trackedIter]struct{}
	stopped   []*trackedIter

	haltChan chan struct{}
	done     chan struct{}
}

func newTracker(buf int) iteratorTracker {
	return iteratorTracker{
		startChan: make(chan *trackedIter, buf),
		stopChan:  make(chan *trackedIter, buf),
		started:   map[*trackedIter]struct{}{},
		haltChan:  make(chan struct{}),
		done:      make(chan struct{}),
	}
}

// listens for starts/stops and manages current state
func (tr *iteratorTracker) run() {
loop:
	for {
		select {
		case start := <-tr.startChan:
			tr.started[start] = struct{}{}
		case stop := <-tr.stopChan:
			tr.stopped = append(tr.stopped, stop)
		case <-tr.haltChan:
			break loop
		default:
		}
	}
	tr.done <- struct{}{}
}

func (tr *iteratorTracker) tracked(it trie.NodeIterator) (ret *trackedIter) {
	ret = &trackedIter{it, tr}
	tr.startChan <- ret
	return
}

// dumps iterator path and bounds to a text file so it can be restored later
func (tr *iteratorTracker) dump(path string) error {
	var rows [][]string
	for it, _ := range tr.started {
		var endPath []byte
		if impl, ok := it.NodeIterator.(*iter.PrefixBoundIterator); ok {
			endPath = impl.EndPath
		}
		rows = append(rows, []string{
			fmt.Sprintf("%x", it.Path()),
			fmt.Sprintf("%x", endPath),
		})
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	out := csv.NewWriter(file)
	return out.WriteAll(rows)
}

// attempts to read iterator state from file
// if file doesn't exist, returns an empty slice with no error
func (tr *iteratorTracker) restore(tree state.Trie, path string) ([]trie.NodeIterator, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer file.Close()
	in := csv.NewReader(file)
	in.FieldsPerRecord = 2
	rows, err := in.ReadAll()
	if err != nil {
		return nil, err
	}
	var ret []trie.NodeIterator
	for _, row := range rows {
		// pick up where each interval left off
		var paths [2][]byte
		for i, val := range row {
			if len(val) != 0 {
				if _, err = fmt.Sscanf(val, "%x", &paths[i]); err != nil {
					return nil, err
				}
			}
		}

		it := iter.NewPrefixBoundIterator(tree, paths[0], paths[1])
		ret = append(ret, tr.tracked(it))
	}
	return ret, nil
}

func (tr *iteratorTracker) haltAndDump(path string) error {
	tr.haltChan <- struct{}{}
	<-tr.done

	// drain any pending events
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
		err := os.Remove(path)
		if os.IsNotExist(err) {
			err = nil
		}
		return err
	}
	return tr.dump(path)
}
