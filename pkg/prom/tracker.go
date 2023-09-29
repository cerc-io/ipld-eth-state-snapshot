package prom

import (
	"fmt"
	"sync"
	"sync/atomic"

	iterutil "github.com/cerc-io/eth-iterator-utils"
	"github.com/cerc-io/eth-iterator-utils/tracker"
	"github.com/ethereum/go-ethereum/trie"
)

var trackedIterCount atomic.Int32

// Tracker which wraps a tracked iterators in metrics-reporting iterators
type MetricsTracker struct {
	*tracker.TrackerImpl
}

type metricsIterator struct {
	trie.NodeIterator
	id int32
	// count    uint
	done     bool
	lastPath []byte
	sync.RWMutex
}

func NewTracker(file string, bufsize uint) *MetricsTracker {
	return &MetricsTracker{TrackerImpl: tracker.NewImpl(file, bufsize)}
}

func (t *MetricsTracker) wrap(tracked *tracker.Iterator) *metricsIterator {
	startPath, endPath := tracked.Bounds()
	startDepth := max(len(startPath), len(endPath))
	ret := &metricsIterator{
		NodeIterator: tracked,
		id:           trackedIterCount.Add(1),
	}
	RegisterGaugeFunc(
		fmt.Sprintf("tracked_iterator_%d", ret.id),
		func() float64 {
			ret.RLock()
			if ret.done {
				return 1
			}
			lastPath := ret.lastPath
			ret.RUnlock()
			if lastPath == nil {
				return 0
			}
			// estimate remaining distance based on current position and node count
			depth := max(startDepth, len(lastPath))
			startPath := normalizePath(startPath, depth)
			endPath := normalizePath(endPath, depth)
			progressed := subtractPaths(lastPath, startPath)
			total := subtractPaths(endPath, startPath)
			return float64(countSteps(progressed, depth)) / float64(countSteps(total, depth))
		})
	return ret
}

func (t *MetricsTracker) Restore(ctor iterutil.IteratorConstructor) (
	[]trie.NodeIterator, []trie.NodeIterator, error,
) {
	iters, bases, err := t.TrackerImpl.Restore(ctor)
	if err != nil {
		return nil, nil, err
	}
	ret := make([]trie.NodeIterator, len(iters))
	for i, tracked := range iters {
		ret[i] = t.wrap(tracked)
	}
	return ret, bases, nil
}

func (t *MetricsTracker) Tracked(it trie.NodeIterator) trie.NodeIterator {
	tracked := t.TrackerImpl.Tracked(it)
	return t.wrap(tracked)
}

func (it *metricsIterator) Next(descend bool) bool {
	ret := it.NodeIterator.Next(descend)
	it.Lock()
	defer it.Unlock()
	if ret {
		it.lastPath = it.Path()
	} else {
		it.done = true
	}
	return ret
}

func normalizePath(path []byte, depth int) []byte {
	normalized := make([]byte, depth)
	for i := 0; i < depth; i++ {
		if i < len(path) {
			normalized[i] = path[i]
		}
	}
	return normalized
}

// Subtract each component, right to left, carrying over if necessary.
func subtractPaths(a, b []byte) []byte {
	diff := make([]byte, len(a))
	carry := false
	for i := len(a) - 1; i >= 0; i-- {
		diff[i] = a[i] - b[i]
		if carry {
			diff[i]--
		}
		carry = a[i] < b[i]
	}
	return diff
}

// count total steps in a path according to its depth (length)
func countSteps(path []byte, depth int) uint {
	var steps uint
	for _, b := range path {
		steps *= 16
		steps += uint(b)
	}
	return steps
}

func max(a int, b int) int {
	if a > b {
		return a
	}
	return b
}
