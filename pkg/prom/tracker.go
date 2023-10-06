package prom

import (
	"bytes"
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
	pathDepth := max(max(len(startPath), len(endPath)), 1)
	totalSteps := estimateSteps(startPath, endPath, pathDepth)

	ret := &metricsIterator{
		NodeIterator: tracked,
		id:           trackedIterCount.Add(1),
	}

	RegisterGaugeFunc(
		fmt.Sprintf("tracked_iterator_%d", ret.id),
		func() float64 {
			ret.RLock()
			done := ret.done
			lastPath := ret.lastPath
			ret.RUnlock()

			if done {
				return 100.0
			}

			if lastPath == nil {
				return 0.0
			}

			// estimate remaining distance based on current position and node count
			remainingSteps := estimateSteps(lastPath, endPath, pathDepth)
			return (float64(totalSteps) - float64(remainingSteps)) / float64(totalSteps) * 100.0
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

// Estimate the number of iterations necessary to step from start to end.
func estimateSteps(start []byte, end []byte, depth int) uint64 {
	// We see paths in several forms (nil, 0600, 06, etc.). We need to adjust them to a comparable form.
	// For nil, start and end indicate the extremes of 0x0 and 0x10.  For differences in depth, we often see a
	// start/end range on a bounded iterator specified like 0500:0600, while the value returned by it.Path() may
	// be shorter, like 06.  Since our goal is to estimate how many steps it would take to move from start to end,
	// we want to perform the comparison at a stable depth, since to move from 05 to 06 is only 1 step, but
	// to move from 0500:06 is 16.
	normalizePathRange := func(start []byte, end []byte, depth int) ([]byte, []byte) {
		if 0 == len(start) {
			start = []byte{0x0}
		}
		if 0 == len(end) {
			end = []byte{0x10}
		}
		normalizedStart := make([]byte, depth)
		normalizedEnd := make([]byte, depth)
		for i := 0; i < depth; i++ {
			if i < len(start) {
				normalizedStart[i] = start[i]
			}
			if i < len(end) {
				normalizedEnd[i] = end[i]
			}
		}
		return normalizedStart, normalizedEnd
	}

	// We have no need to handle negative exponents, so uints are fine.
	pow := func(x uint64, y uint) uint64 {
		if 0 == y {
			return 1
		}
		ret := x
		for i := uint(0); i < y; i++ {
			ret *= x
		}
		return x
	}

	// Fix the paths.
	start, end = normalizePathRange(start, end, depth)

	// No negative distances, if the start is already >= end, the distance is 0.
	if bytes.Compare(start, end) >= 0 {
		return 0
	}

	// Subtract each component, right to left, carrying over if necessary.
	difference := make([]byte, len(start))
	var carry byte = 0
	for i := len(start) - 1; i >= 0; i-- {
		result := end[i] - start[i] - carry
		if result > 0xf && i > 0 {
			result &= 0xf
			carry = 1
		} else {
			carry = 0
		}
		difference[i] = result
	}

	// Calculate the result.
	var ret uint64 = 0
	for i := 0; i < len(difference); i++ {
		ret += uint64(difference[i]) * pow(16, uint(len(difference)-i-1))
	}

	return ret
}

func max(a int, b int) int {
	if a > b {
		return a
	}
	return b
}
