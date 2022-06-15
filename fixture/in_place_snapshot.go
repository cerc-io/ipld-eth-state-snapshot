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

package fixture

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	snapt "github.com/vulcanize/ipld-eth-state-snapshot/pkg/types"
)

type Block struct {
	Hash         common.Hash
	Number       *big.Int
	StateNodes   []snapt.Node
	StorageNodes [][]snapt.Node
}

var InPlaceSnapshotBlocks = []Block{
	// Genesis block
	{
		Hash:   common.HexToHash("0xe1bdb963128f645aa674b52a8c7ce00704762f27e2a6896abebd7954878f40e4"),
		Number: big.NewInt(0),
		StateNodes: []snapt.Node{
			// State node for main account with balance.
			{
				NodeType: 2,
				Path:     []byte{},
				Key:      common.HexToHash("0x67ab3c0dd775f448af7fb41243415ed6fb975d1530a2d828f69bea7346231ad7"),
				Value:    []byte{248, 119, 161, 32, 103, 171, 60, 13, 215, 117, 244, 72, 175, 127, 180, 18, 67, 65, 94, 214, 251, 151, 93, 21, 48, 162, 216, 40, 246, 155, 234, 115, 70, 35, 26, 215, 184, 83, 248, 81, 128, 141, 2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 160, 86, 232, 31, 23, 27, 204, 85, 166, 255, 131, 69, 230, 146, 192, 248, 110, 91, 72, 224, 27, 153, 108, 173, 192, 1, 98, 47, 181, 227, 99, 180, 33, 160, 197, 210, 70, 1, 134, 247, 35, 60, 146, 126, 125, 178, 220, 199, 3, 192, 229, 0, 182, 83, 202, 130, 39, 59, 123, 250, 216, 4, 93, 133, 164, 112},
			},
		},
	},
	// Contract Test1 deployed by main account.
	{
		Hash:   common.HexToHash("0x46ce57b700e470d0c0820ede662ecc0d0c78cf87237cb12a40a7ff5ff9cc8ac5"),
		Number: big.NewInt(1),
		StateNodes: []snapt.Node{
			// Branch root node.
			{
				NodeType: 0,
				Path:     []byte{},
				Value:    []byte{248, 81, 128, 128, 128, 128, 128, 128, 160, 173, 52, 73, 195, 118, 160, 81, 100, 138, 50, 127, 27, 188, 85, 147, 215, 187, 244, 219, 228, 93, 25, 72, 253, 160, 45, 16, 239, 130, 223, 160, 26, 128, 128, 160, 137, 52, 229, 60, 211, 96, 171, 177, 51, 19, 204, 180, 24, 252, 28, 70, 234, 7, 73, 20, 117, 230, 32, 223, 188, 6, 191, 75, 123, 64, 163, 197, 128, 128, 128, 128, 128, 128, 128},
			},
			// State node for sender account.
			{
				NodeType: 2,
				Path:     []byte{6},
				Key:      common.HexToHash("0x67ab3c0dd775f448af7fb41243415ed6fb975d1530a2d828f69bea7346231ad7"),
				Value:    []byte{248, 118, 160, 55, 171, 60, 13, 215, 117, 244, 72, 175, 127, 180, 18, 67, 65, 94, 214, 251, 151, 93, 21, 48, 162, 216, 40, 246, 155, 234, 115, 70, 35, 26, 215, 184, 83, 248, 81, 1, 141, 2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 160, 86, 232, 31, 23, 27, 204, 85, 166, 255, 131, 69, 230, 146, 192, 248, 110, 91, 72, 224, 27, 153, 108, 173, 192, 1, 98, 47, 181, 227, 99, 180, 33, 160, 197, 210, 70, 1, 134, 247, 35, 60, 146, 126, 125, 178, 220, 199, 3, 192, 229, 0, 182, 83, 202, 130, 39, 59, 123, 250, 216, 4, 93, 133, 164, 112},
			},
			// State node for deployment of contract Test1.
			{
				NodeType: 2,
				Path:     []byte{9},
				Key:      common.HexToHash("0x9397e33dedda4763aea143fc6151ebcd9a93f62db7a6a556d46c585d82ad2afc"),
				Value:    []byte{248, 105, 160, 51, 151, 227, 61, 237, 218, 71, 99, 174, 161, 67, 252, 97, 81, 235, 205, 154, 147, 246, 45, 183, 166, 165, 86, 212, 108, 88, 93, 130, 173, 42, 252, 184, 70, 248, 68, 1, 128, 160, 243, 143, 159, 99, 199, 96, 208, 136, 215, 221, 4, 247, 67, 97, 155, 98, 145, 246, 59, 238, 189, 139, 223, 83, 6, 40, 249, 14, 156, 250, 82, 215, 160, 47, 62, 207, 242, 160, 167, 130, 233, 6, 187, 196, 80, 96, 6, 188, 150, 74, 176, 201, 7, 65, 32, 174, 97, 1, 76, 26, 86, 141, 49, 62, 214},
			},
		},
		StorageNodes: [][]snapt.Node{
			{},
			{},
			{
				// Storage node for contract Test1 state variable initialCount.
				{
					NodeType: 2,
					Path:     []byte{},
					Key:      common.HexToHash("0xb10e2d527612073b26eecdfd717e6a320cf44b4afac2b0732d9fcbe2b7fa0cf6"),
					Value:    []byte{227, 161, 32, 177, 14, 45, 82, 118, 18, 7, 59, 38, 238, 205, 253, 113, 126, 106, 50, 12, 244, 75, 74, 250, 194, 176, 115, 45, 159, 203, 226, 183, 250, 12, 246, 1},
				},
			},
		},
	},
	// Contract Test2 deployed by main account.
	{
		Hash:   common.HexToHash("0xa848b156fe4e61d8dac0a833720794e8c58e93fa6db369af6f0d9a19ada9d723"),
		Number: big.NewInt(2),
		StateNodes: []snapt.Node{
			// Branch root node.
			{
				NodeType: 0,
				Path:     []byte{},
				Value:    []byte{248, 81, 128, 128, 128, 128, 128, 128, 160, 191, 248, 9, 223, 101, 212, 255, 213, 196, 146, 160, 239, 69, 178, 134, 139, 81, 22, 255, 149, 90, 253, 178, 172, 102, 87, 249, 225, 224, 173, 183, 55, 128, 128, 160, 165, 200, 234, 64, 112, 157, 130, 31, 236, 38, 20, 68, 99, 247, 81, 161, 76, 62, 186, 246, 84, 121, 39, 155, 102, 134, 188, 109, 89, 220, 31, 212, 128, 128, 128, 128, 128, 128, 128},
			},
			// State node for sender account.
			{
				NodeType: 2,
				Path:     []byte{6},
				Key:      common.HexToHash("0x67ab3c0dd775f448af7fb41243415ed6fb975d1530a2d828f69bea7346231ad7"),
				Value:    []byte{248, 118, 160, 55, 171, 60, 13, 215, 117, 244, 72, 175, 127, 180, 18, 67, 65, 94, 214, 251, 151, 93, 21, 48, 162, 216, 40, 246, 155, 234, 115, 70, 35, 26, 215, 184, 83, 248, 81, 2, 141, 2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 160, 86, 232, 31, 23, 27, 204, 85, 166, 255, 131, 69, 230, 146, 192, 248, 110, 91, 72, 224, 27, 153, 108, 173, 192, 1, 98, 47, 181, 227, 99, 180, 33, 160, 197, 210, 70, 1, 134, 247, 35, 60, 146, 126, 125, 178, 220, 199, 3, 192, 229, 0, 182, 83, 202, 130, 39, 59, 123, 250, 216, 4, 93, 133, 164, 112},
			},
			// State node for deployment of contract Test2.
			{
				NodeType: 2,
				Path:     []byte{10},
				Key:      common.HexToHash("0xa44b5f4b47ded891709350af6a6e4d56602228a70279bdad4f0f64042445b4b9"),
				Value:    []byte{248, 105, 160, 52, 75, 95, 75, 71, 222, 216, 145, 112, 147, 80, 175, 106, 110, 77, 86, 96, 34, 40, 167, 2, 121, 189, 173, 79, 15, 100, 4, 36, 69, 180, 185, 184, 70, 248, 68, 1, 128, 160, 130, 30, 37, 86, 162, 144, 200, 100, 5, 248, 22, 10, 45, 102, 32, 66, 164, 49, 186, 69, 107, 157, 178, 101, 199, 155, 184, 55, 192, 75, 229, 240, 160, 86, 36, 245, 233, 5, 167, 42, 118, 181, 35, 178, 216, 149, 56, 146, 147, 19, 8, 140, 137, 234, 0, 160, 27, 220, 33, 204, 6, 152, 239, 177, 52},
			},
		},
		StorageNodes: [][]snapt.Node{
			{},
			{},
			{
				// Storage node for contract Test2 state variable test.
				{
					NodeType: 2,
					Path:     []byte{},
					Key:      common.HexToHash("0x290decd9548b62a8d60345a988386fc84ba6bc95484008f6362f93160ef3e563"),
					Value:    []byte{227, 161, 32, 41, 13, 236, 217, 84, 139, 98, 168, 214, 3, 69, 169, 136, 56, 111, 200, 75, 166, 188, 149, 72, 64, 8, 246, 54, 47, 147, 22, 14, 243, 229, 99, 1},
				},
			},
		},
	},
	// Increment contract Test1 state variable count using main account.
	{
		Hash:   common.HexToHash("0x9fc4aaaab26f0b43ac609c99ae50925e5dc9a25f103c0511fcff38c6b3158302"),
		Number: big.NewInt(3),
		StateNodes: []snapt.Node{
			// Branch root node.
			{
				NodeType: 0,
				Path:     []byte{},
				Value:    []byte{248, 113, 128, 128, 128, 128, 128, 128, 160, 70, 53, 190, 199, 124, 254, 86, 213, 42, 126, 117, 155, 2, 223, 56, 167, 130, 118, 10, 150, 65, 46, 207, 169, 167, 250, 209, 64, 37, 205, 153, 51, 128, 128, 160, 165, 200, 234, 64, 112, 157, 130, 31, 236, 38, 20, 68, 99, 247, 81, 161, 76, 62, 186, 246, 84, 121, 39, 155, 102, 134, 188, 109, 89, 220, 31, 212, 128, 128, 160, 214, 109, 199, 206, 145, 11, 213, 44, 206, 214, 36, 181, 134, 92, 243, 178, 58, 88, 158, 42, 31, 125, 71, 148, 188, 122, 252, 100, 250, 182, 85, 159, 128, 128, 128, 128},
			},
			// State node for sender account.
			{
				NodeType: 2,
				Path:     []byte{6},
				Key:      common.HexToHash("0x67ab3c0dd775f448af7fb41243415ed6fb975d1530a2d828f69bea7346231ad7"),
				Value:    []byte{248, 118, 160, 55, 171, 60, 13, 215, 117, 244, 72, 175, 127, 180, 18, 67, 65, 94, 214, 251, 151, 93, 21, 48, 162, 216, 40, 246, 155, 234, 115, 70, 35, 26, 215, 184, 83, 248, 81, 3, 141, 2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 160, 86, 232, 31, 23, 27, 204, 85, 166, 255, 131, 69, 230, 146, 192, 248, 110, 91, 72, 224, 27, 153, 108, 173, 192, 1, 98, 47, 181, 227, 99, 180, 33, 160, 197, 210, 70, 1, 134, 247, 35, 60, 146, 126, 125, 178, 220, 199, 3, 192, 229, 0, 182, 83, 202, 130, 39, 59, 123, 250, 216, 4, 93, 133, 164, 112},
			},
			// State node for contract Test1 transaction.
			{
				NodeType: 2,
				Path:     []byte{9},
				Key:      common.HexToHash("0x9397e33dedda4763aea143fc6151ebcd9a93f62db7a6a556d46c585d82ad2afc"),
				Value:    []byte{248, 105, 160, 51, 151, 227, 61, 237, 218, 71, 99, 174, 161, 67, 252, 97, 81, 235, 205, 154, 147, 246, 45, 183, 166, 165, 86, 212, 108, 88, 93, 130, 173, 42, 252, 184, 70, 248, 68, 1, 128, 160, 167, 171, 204, 110, 30, 52, 74, 189, 215, 97, 245, 227, 176, 141, 250, 205, 8, 182, 138, 101, 51, 150, 155, 174, 234, 246, 30, 128, 253, 230, 36, 228, 160, 47, 62, 207, 242, 160, 167, 130, 233, 6, 187, 196, 80, 96, 6, 188, 150, 74, 176, 201, 7, 65, 32, 174, 97, 1, 76, 26, 86, 141, 49, 62, 214},
			},
		},
		StorageNodes: [][]snapt.Node{
			{},
			{},
			{
				// Branch root node.
				{
					NodeType: 0,
					Path:     []byte{},
					Value:    []byte{248, 81, 128, 128, 160, 79, 197, 241, 58, 178, 249, 186, 12, 45, 168, 139, 1, 81, 171, 14, 124, 244, 216, 93, 8, 204, 164, 92, 205, 146, 60, 106, 183, 99, 35, 235, 40, 128, 128, 128, 128, 128, 128, 128, 128, 160, 244, 152, 74, 17, 246, 26, 41, 33, 69, 97, 65, 223, 136, 222, 110, 26, 113, 13, 40, 104, 27, 145, 175, 121, 76, 90, 114, 30, 71, 131, 156, 215, 128, 128, 128, 128, 128},
				},
				// Storage node for contract Test1 state variable count.
				{
					NodeType: 2,
					Path:     []byte{2},
					Key:      common.HexToHash("0x290decd9548b62a8d60345a988386fc84ba6bc95484008f6362f93160ef3e563"),
					Value:    []byte{226, 160, 57, 13, 236, 217, 84, 139, 98, 168, 214, 3, 69, 169, 136, 56, 111, 200, 75, 166, 188, 149, 72, 64, 8, 246, 54, 47, 147, 22, 14, 243, 229, 99, 1},
				},
				// Storage node for contract Test1 state variable initialCount.
				{
					NodeType: 2,
					Path:     []byte{11},
					Key:      common.HexToHash("0xb10e2d527612073b26eecdfd717e6a320cf44b4afac2b0732d9fcbe2b7fa0cf6"),
					Value:    []byte{226, 160, 49, 14, 45, 82, 118, 18, 7, 59, 38, 238, 205, 253, 113, 126, 106, 50, 12, 244, 75, 74, 250, 194, 176, 115, 45, 159, 203, 226, 183, 250, 12, 246, 1},
				},
			},
		},
	},
	// Destory contracts Test1 and Test2.
	{
		Hash:   common.HexToHash("0xa1f10477ca69841685d5fdc24214767379b937b66a17f005d4352d5a783e8982"),
		Number: big.NewInt(4),
		StateNodes: []snapt.Node{
			// State node for sender account.
			{
				NodeType: 2,
				Key:      common.HexToHash("0x67ab3c0dd775f448af7fb41243415ed6fb975d1530a2d828f69bea7346231ad7"),
				Path:     []byte{},
				Value:    []byte{248, 119, 161, 32, 103, 171, 60, 13, 215, 117, 244, 72, 175, 127, 180, 18, 67, 65, 94, 214, 251, 151, 93, 21, 48, 162, 216, 40, 246, 155, 234, 115, 70, 35, 26, 215, 184, 83, 248, 81, 5, 141, 2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 160, 86, 232, 31, 23, 27, 204, 85, 166, 255, 131, 69, 230, 146, 192, 248, 110, 91, 72, 224, 27, 153, 108, 173, 192, 1, 98, 47, 181, 227, 99, 180, 33, 160, 197, 210, 70, 1, 134, 247, 35, 60, 146, 126, 125, 178, 220, 199, 3, 192, 229, 0, 182, 83, 202, 130, 39, 59, 123, 250, 216, 4, 93, 133, 164, 112},
			},
			// Branch node for removed type.
			{
				NodeType: 3,
				Path:     []byte{6},
				Value:    []byte{},
			},
			// State node for destroying contract Test1.
			{
				NodeType: 3,
				Path:     []byte{9},
				Key:      common.HexToHash("0x9397e33dedda4763aea143fc6151ebcd9a93f62db7a6a556d46c585d82ad2afc"),
				Value:    []byte{},
			},
			// State node for destroying contract Test2.
			{
				NodeType: 3,
				Path:     []byte{10},
				Key:      common.HexToHash("0xa44b5f4b47ded891709350af6a6e4d56602228a70279bdad4f0f64042445b4b9"),
				Value:    []byte{},
			},
		},
		StorageNodes: [][]snapt.Node{
			{},
			{},
			{
				// Branch node for removed type.
				{
					NodeType: 3,
					Path:     []byte{},
					Value:    []byte{},
				},
				// Storage node for contract Test1 state variable count.
				{
					NodeType: 3,
					Path:     []byte{2},
					Key:      common.HexToHash("0x290decd9548b62a8d60345a988386fc84ba6bc95484008f6362f93160ef3e563"),
					Value:    []byte{},
				},
				// Storage node for contract Test1 state variable initialCount.
				{
					NodeType: 3,
					Path:     []byte{11},
					Key:      common.HexToHash("0xb10e2d527612073b26eecdfd717e6a320cf44b4afac2b0732d9fcbe2b7fa0cf6"),
					Value:    []byte{},
				},
			},
			{
				// Storage node for contract Test2 state variable test.
				{
					NodeType: 3,
					Path:     []byte{},
					Key:      common.HexToHash("0x290decd9548b62a8d60345a988386fc84ba6bc95484008f6362f93160ef3e563"),
					Value:    []byte{},
				},
			},
		},
	},
}

type StorageNodeWithState struct {
	snapt.Node
	StatePath []byte
}

// Expected state nodes at snapshot height.
var ExpectedStateNodes = []snapt.Node{
	{
		NodeType: 0,
		Path:     []byte{},
		Value:    []byte{248, 113, 128, 128, 128, 128, 128, 128, 160, 70, 53, 190, 199, 124, 254, 86, 213, 42, 126, 117, 155, 2, 223, 56, 167, 130, 118, 10, 150, 65, 46, 207, 169, 167, 250, 209, 64, 37, 205, 153, 51, 128, 128, 160, 165, 200, 234, 64, 112, 157, 130, 31, 236, 38, 20, 68, 99, 247, 81, 161, 76, 62, 186, 246, 84, 121, 39, 155, 102, 134, 188, 109, 89, 220, 31, 212, 128, 128, 160, 214, 109, 199, 206, 145, 11, 213, 44, 206, 214, 36, 181, 134, 92, 243, 178, 58, 88, 158, 42, 31, 125, 71, 148, 188, 122, 252, 100, 250, 182, 85, 159, 128, 128, 128, 128},
	},
	{
		NodeType: 2,
		Path:     []byte{6},
		Key:      common.HexToHash("0x67ab3c0dd775f448af7fb41243415ed6fb975d1530a2d828f69bea7346231ad7"),
		Value:    []byte{248, 118, 160, 55, 171, 60, 13, 215, 117, 244, 72, 175, 127, 180, 18, 67, 65, 94, 214, 251, 151, 93, 21, 48, 162, 216, 40, 246, 155, 234, 115, 70, 35, 26, 215, 184, 83, 248, 81, 3, 141, 2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 160, 86, 232, 31, 23, 27, 204, 85, 166, 255, 131, 69, 230, 146, 192, 248, 110, 91, 72, 224, 27, 153, 108, 173, 192, 1, 98, 47, 181, 227, 99, 180, 33, 160, 197, 210, 70, 1, 134, 247, 35, 60, 146, 126, 125, 178, 220, 199, 3, 192, 229, 0, 182, 83, 202, 130, 39, 59, 123, 250, 216, 4, 93, 133, 164, 112},
	},
	{
		NodeType: 2,
		Path:     []byte{9},
		Key:      common.HexToHash("0x9397e33dedda4763aea143fc6151ebcd9a93f62db7a6a556d46c585d82ad2afc"),
		Value:    []byte{248, 105, 160, 51, 151, 227, 61, 237, 218, 71, 99, 174, 161, 67, 252, 97, 81, 235, 205, 154, 147, 246, 45, 183, 166, 165, 86, 212, 108, 88, 93, 130, 173, 42, 252, 184, 70, 248, 68, 1, 128, 160, 167, 171, 204, 110, 30, 52, 74, 189, 215, 97, 245, 227, 176, 141, 250, 205, 8, 182, 138, 101, 51, 150, 155, 174, 234, 246, 30, 128, 253, 230, 36, 228, 160, 47, 62, 207, 242, 160, 167, 130, 233, 6, 187, 196, 80, 96, 6, 188, 150, 74, 176, 201, 7, 65, 32, 174, 97, 1, 76, 26, 86, 141, 49, 62, 214},
	},
	{
		NodeType: 2,
		Path:     []byte{10},
		Key:      common.HexToHash("0xa44b5f4b47ded891709350af6a6e4d56602228a70279bdad4f0f64042445b4b9"),
		Value:    []byte{248, 105, 160, 52, 75, 95, 75, 71, 222, 216, 145, 112, 147, 80, 175, 106, 110, 77, 86, 96, 34, 40, 167, 2, 121, 189, 173, 79, 15, 100, 4, 36, 69, 180, 185, 184, 70, 248, 68, 1, 128, 160, 130, 30, 37, 86, 162, 144, 200, 100, 5, 248, 22, 10, 45, 102, 32, 66, 164, 49, 186, 69, 107, 157, 178, 101, 199, 155, 184, 55, 192, 75, 229, 240, 160, 86, 36, 245, 233, 5, 167, 42, 118, 181, 35, 178, 216, 149, 56, 146, 147, 19, 8, 140, 137, 234, 0, 160, 27, 220, 33, 204, 6, 152, 239, 177, 52},
	},
}

// Expected storage nodes at snapshot height.
var ExpectedStorageNodes = []StorageNodeWithState{
	{
		Node: snapt.Node{
			NodeType: 0,
			Path:     []byte{},
			Value:    []byte{248, 81, 128, 128, 160, 79, 197, 241, 58, 178, 249, 186, 12, 45, 168, 139, 1, 81, 171, 14, 124, 244, 216, 93, 8, 204, 164, 92, 205, 146, 60, 106, 183, 99, 35, 235, 40, 128, 128, 128, 128, 128, 128, 128, 128, 160, 244, 152, 74, 17, 246, 26, 41, 33, 69, 97, 65, 223, 136, 222, 110, 26, 113, 13, 40, 104, 27, 145, 175, 121, 76, 90, 114, 30, 71, 131, 156, 215, 128, 128, 128, 128, 128},
		},
		StatePath: []byte{9},
	},
	{
		Node: snapt.Node{
			NodeType: 2,
			Path:     []byte{2},
			Key:      common.HexToHash("0x290decd9548b62a8d60345a988386fc84ba6bc95484008f6362f93160ef3e563"),
			Value:    []byte{226, 160, 57, 13, 236, 217, 84, 139, 98, 168, 214, 3, 69, 169, 136, 56, 111, 200, 75, 166, 188, 149, 72, 64, 8, 246, 54, 47, 147, 22, 14, 243, 229, 99, 1},
		},
		StatePath: []byte{9},
	},
	{
		Node: snapt.Node{
			NodeType: 2,
			Path:     []byte{11},
			Key:      common.HexToHash("0xb10e2d527612073b26eecdfd717e6a320cf44b4afac2b0732d9fcbe2b7fa0cf6"),
			Value:    []byte{226, 160, 49, 14, 45, 82, 118, 18, 7, 59, 38, 238, 205, 253, 113, 126, 106, 50, 12, 244, 75, 74, 250, 194, 176, 115, 45, 159, 203, 226, 183, 250, 12, 246, 1},
		},
		StatePath: []byte{9},
	},
	{
		Node: snapt.Node{
			NodeType: 2,
			Path:     []byte{},
			Key:      common.HexToHash("0x290decd9548b62a8d60345a988386fc84ba6bc95484008f6362f93160ef3e563"),
			Value:    []byte{227, 161, 32, 41, 13, 236, 217, 84, 139, 98, 168, 214, 3, 69, 169, 136, 56, 111, 200, 75, 166, 188, 149, 72, 64, 8, 246, 54, 47, 147, 22, 14, 243, 229, 99, 1},
		},
		StatePath: []byte{10},
	},
}

// Expected state nodes at snapshot height after destroying contracts.
var ExpectedStateNodesAfterContractDestruction = []snapt.Node{
	{
		NodeType: 2,
		Path:     []byte{},
		Key:      common.HexToHash("0x67ab3c0dd775f448af7fb41243415ed6fb975d1530a2d828f69bea7346231ad7"),
		Value:    []byte{248, 119, 161, 32, 103, 171, 60, 13, 215, 117, 244, 72, 175, 127, 180, 18, 67, 65, 94, 214, 251, 151, 93, 21, 48, 162, 216, 40, 246, 155, 234, 115, 70, 35, 26, 215, 184, 83, 248, 81, 5, 141, 2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 160, 86, 232, 31, 23, 27, 204, 85, 166, 255, 131, 69, 230, 146, 192, 248, 110, 91, 72, 224, 27, 153, 108, 173, 192, 1, 98, 47, 181, 227, 99, 180, 33, 160, 197, 210, 70, 1, 134, 247, 35, 60, 146, 126, 125, 178, 220, 199, 3, 192, 229, 0, 182, 83, 202, 130, 39, 59, 123, 250, 216, 4, 93, 133, 164, 112},
	},
	{
		NodeType: 3,
		Path:     []byte{6},
		Value:    []byte{},
	},
	{
		NodeType: 3,
		Path:     []byte{9},
		Key:      common.HexToHash("0x9397e33dedda4763aea143fc6151ebcd9a93f62db7a6a556d46c585d82ad2afc"),
		Value:    []byte{},
	},
	{
		NodeType: 3,
		Path:     []byte{10},
		Key:      common.HexToHash("0xa44b5f4b47ded891709350af6a6e4d56602228a70279bdad4f0f64042445b4b9"),
		Value:    []byte{},
	},
}

// Expected storage nodes at snapshot height after destroying contracts..
var ExpectedStorageNodesAfterContractDestruction = []StorageNodeWithState{
	{
		Node: snapt.Node{
			NodeType: 3,
			Path:     []byte{},
			Value:    []byte{},
		},
		StatePath: []byte{9},
	},
	{
		Node: snapt.Node{
			NodeType: 3,
			Path:     []byte{2},
			Key:      common.HexToHash("0x290decd9548b62a8d60345a988386fc84ba6bc95484008f6362f93160ef3e563"),
			Value:    []byte{},
		},
		StatePath: []byte{9},
	},
	{
		Node: snapt.Node{
			NodeType: 3,
			Path:     []byte{11},
			Key:      common.HexToHash("0xb10e2d527612073b26eecdfd717e6a320cf44b4afac2b0732d9fcbe2b7fa0cf6"),
			Value:    []byte{},
		},
		StatePath: []byte{9},
	},
	{
		Node: snapt.Node{
			NodeType: 3,
			Path:     []byte{},
			Key:      common.HexToHash("0x290decd9548b62a8d60345a988386fc84ba6bc95484008f6362f93160ef3e563"),
			Value:    []byte{},
		},
		StatePath: []byte{10},
	},
}

// Block header at snapshot height.
// Required in database when executing inPlaceStateSnapshot.
var Block5_Header = types.Header{
	ParentHash:  common.HexToHash("0xa1f10477ca69841685d5fdc24214767379b937b66a17f005d4352d5a783e8982"),
	UncleHash:   common.HexToHash("0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347"),
	Coinbase:    common.HexToAddress("0x0000000000000000000000000000000000000000"),
	Root:        common.HexToHash("0x53580584816f617295ea26c0e17641e0120cab2f0a8ffb53a866fd53aa8e8c2d"),
	TxHash:      common.HexToHash("0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421"),
	ReceiptHash: common.HexToHash("0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421"),
	Bloom:       types.Bloom{},
	Difficulty:  big.NewInt(+2),
	Number:      big.NewInt(5),
	GasLimit:    4704588,
	GasUsed:     0,
	Time:        1492010458,
	Extra:       []byte{215, 131, 1, 6, 0, 132, 103, 101, 116, 104, 135, 103, 111, 49, 46, 55, 46, 51, 133, 108, 105, 110, 117, 120, 0, 0, 0, 0, 0, 0, 0, 0, 159, 30, 250, 30, 250, 114, 175, 19, 140, 145, 89, 102, 198, 57, 84, 74, 2, 85, 230, 40, 142, 24, 140, 34, 206, 145, 104, 193, 13, 190, 70, 218, 61, 136, 180, 170, 6, 89, 48, 17, 159, 184, 134, 33, 11, 240, 26, 8, 79, 222, 93, 59, 196, 141, 138, 163, 139, 202, 146, 228, 252, 197, 33, 81, 0},
	MixDigest:   common.Hash{},
	Nonce:       types.BlockNonce{},
	BaseFee:     nil,
}

/*
pragma solidity ^0.8.0;

contract Test1 {
    uint256 private count;
    uint256 private initialCount;

    event Increment(uint256 count);

    constructor() {
      initialCount = 1;
    }

    function incrementCount() public returns (uint256) {
      count = count + 1;
      emit Increment(count);

      return count;
    }
}
*/

/*
pragma solidity ^0.8.0;

contract Test2 {
    uint256 private test;

    constructor() {
    	test = 1;
    }
}
*/
