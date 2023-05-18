package snapshot

import (
	"bytes"
	"context"
	"fmt"

	"github.com/cerc-io/ipld-eth-state-snapshot/pkg/prom"
	file "github.com/cerc-io/ipld-eth-state-snapshot/pkg/snapshot/file"
	"github.com/cerc-io/ipld-eth-state-snapshot/pkg/snapshot/pg"
	snapt "github.com/cerc-io/ipld-eth-state-snapshot/pkg/types"
	"github.com/ethereum/go-ethereum/statediff/indexer/database/sql/postgres"
)

func NewPublisher(mode SnapshotMode, config *Config) (snapt.Publisher, error) {
	switch mode {
	case PgSnapshot:
		driver, err := postgres.NewPGXDriver(context.Background(), config.DB.ConnConfig, config.Eth.NodeInfo)
		if err != nil {
			return nil, err
		}

		prom.RegisterDBCollector(config.DB.ConnConfig.DatabaseName, driver)

		return pg.NewPublisher(postgres.NewPostgresDB(driver, false)), nil
	case FileSnapshot:
		return file.NewPublisher(config.File.OutputDir, config.Eth.NodeInfo)
	}
	return nil, fmt.Errorf("invalid snapshot mode: %s", mode)
}

// Subtracts 1 from the last byte in a path slice, carrying if needed.
// Does nothing, returning false, for all-zero inputs.
func decrementPath(path []byte) bool {
	// check for all zeros
	allzero := true
	for i := 0; i < len(path); i++ {
		allzero = allzero && path[i] == 0
	}
	if allzero {
		return false
	}
	for i := len(path) - 1; i >= 0; i-- {
		val := path[i]
		path[i]--
		if val == 0 {
			path[i] = 0xf
		} else {
			return true
		}
	}
	return true
}

// Return the distance between two paths.  This is the "distance to go" from start to end and the result is
// equivalent to counting the number of times decrementPath(end) would need to be called before end <= start.
// If end is already <= start, the result is 0.
func pathDistance(start []byte, end []byte) uint64 {
	// We see paths in several forms, nil, 0600, 06, etc. We need to coerce them into a comparable form.
	// For nil, start and end mean the extremes of 0x0 and 0x10.  For differences in length, we sometimes see a
	// start/end range on a bounded iterator specified like 0500:0600, while the value returned by it.Path() may
	// be shorter, like 06.  For the kind of comparison we are doing here, 06 and 0600 should be treated such
	// that 05:06 = 0500:0600 = 16.
	normalizePathRange := func(start []byte, end []byte) ([]byte, []byte) {
		if 0 == len(start) {
			start = []byte{0x0}
		}
		if 0 == len(end) {
			end = []byte{0x10}
		}
		if len(start) == len(end) {
			return start, end
		}
		maxLen := len(end)
		if len(start) > len(end) {
			maxLen = len(start)
		}
		normalizedStart := make([]byte, maxLen)
		normalizedEnd := make([]byte, maxLen)
		for i := 0; i < maxLen; i++ {
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
	start, end = normalizePathRange(start, end)

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

// https://github.com/ethereum/go-ethereum/blob/master/trie/encoding.go#L97
func keybytesToHex(str []byte) []byte {
	l := len(str)*2 + 1
	var nibbles = make([]byte, l)
	for i, b := range str {
		nibbles[i*2] = b / 16
		nibbles[i*2+1] = b % 16
	}
	nibbles[l-1] = 16
	return nibbles
}

func updateSeekedPath(seekedPath *[]byte, nodePath []byte) {
	// assumes len(nodePath) <= max len(*seekedPath)
	*seekedPath = (*seekedPath)[:len(nodePath)]
	copy(*seekedPath, nodePath)
}

// checks that the provided node path is before the end path
func checkUpperPathBound(nodePath, endPath []byte) bool {
	// every path is before nil endPath
	if endPath == nil {
		return true
	}

	if len(endPath)%2 == 0 {
		// in case of even length endpath
		// apply open interval filter since the node at endpath will be covered by the next iterator
		return bytes.Compare(nodePath, endPath) < 0
	}

	return bytes.Compare(nodePath, endPath) <= 0
}
