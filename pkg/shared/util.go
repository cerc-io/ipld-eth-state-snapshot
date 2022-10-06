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

package shared

import (
	"bytes"
)

// DecrementPath subtracts 1 from the last byte in a path slice, carrying if needed.
// Does nothing, returning false, for all-zero inputs.
func DecrementPath(path []byte) bool {
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

// KeyBytesToHex converts key bytes to hex represenation https://github.com/ethereum/go-ethereum/blob/master/trie/encoding.go#L97
func KeyBytesToHex(str []byte) []byte {
	l := len(str)*2 + 1
	var nibbles = make([]byte, l)
	for i, b := range str {
		nibbles[i*2] = b / 16
		nibbles[i*2+1] = b % 16
	}
	nibbles[l-1] = 16
	return nibbles
}

// UpdateSeekedPath updates the seeked paths
func UpdateSeekedPath(seekedPath *[]byte, nodePath []byte) {
	// assumes len(nodePath) <= max len(*seekedPath)
	*seekedPath = (*seekedPath)[:len(nodePath)]
	copy(*seekedPath, nodePath)
}

// CheckUpperPathBound checks that the provided node path is before the end path
func CheckUpperPathBound(nodePath, endPath []byte) bool {
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

// ValidPath checks if a path is prefix to any one of the paths in the given list
func ValidPath(currentPath []byte, seekingPaths [][]byte) bool {
	for _, seekingPath := range seekingPaths {
		if bytes.HasPrefix(seekingPath, currentPath) {
			return true
		}
	}
	return false
}
