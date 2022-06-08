package snapshot

import (
	"encoding/csv"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/trie"
	log "github.com/sirupsen/logrus"

	iter "github.com/vulcanize/go-eth-state-node-iterator"
)

type trackedIter struct {
	trie.NodeIterator
	tracker *iteratorTracker
}

func (it *trackedIter) Next(descend bool) bool {
	ret := it.NodeIterator.Next(descend)
	if !ret {
		if it.tracker.running {
			it.tracker.stopChan <- it
		} else {
			log.Errorf("iterator stopped after tracker halted: path=%x", it.Path())
		}
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
func (tr *iteratorTracker) tracked(it trie.NodeIterator) (ret *trackedIter) {
	ret = &trackedIter{it, tr}
	tr.startChan <- ret
	return
}

// dumps iterator path and bounds to a text file so it can be restored later
func (tr *iteratorTracker) dump() error {
	log.Info("Dumping recovery state to: ", tr.recoveryFile)
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

		// Force the lower bound path to an even length
		if len(paths[0])&0b1 == 1 {
			decrementPath(paths[0]) // decrement first to avoid skipped nodes
			paths[0] = append(paths[0], 0)
		}
		it := iter.NewPrefixBoundIterator(tree.NodeIterator(iter.HexToKeyBytes(paths[0])), paths[1])
		ret = append(ret, tr.tracked(it))
	}
	return ret, nil
}

func (tr *iteratorTracker) haltAndDump() error {
	tr.running = false

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
		err := os.Remove(tr.recoveryFile)
		if os.IsNotExist(err) {
			err = nil
		}
		return err
	}
	return tr.dump()
}
