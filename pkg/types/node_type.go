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

package types

import (
	"fmt"
)

// NodeType for explicitly setting type of node
type NodeType int

const (
	Branch NodeType = iota
	Extension
	Leaf
	Removed
	Unknown
)

// CheckKeyType checks what type of key we have
func CheckKeyType(elements []interface{}) (NodeType, error) {
	if len(elements) > 2 {
		return Branch, nil
	}
	if len(elements) < 2 {
		return Unknown, fmt.Errorf("node cannot be less than two elements in length")
	}
	switch elements[0].([]byte)[0] / 16 {
	case '\x00':
		return Extension, nil
	case '\x01':
		return Extension, nil
	case '\x02':
		return Leaf, nil
	case '\x03':
		return Leaf, nil
	default:
		return Unknown, fmt.Errorf("unknown hex prefix")
	}
}
