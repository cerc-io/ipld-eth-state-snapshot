package fixture

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	// "github.com/jmoiron/sqlx"

	snapt "github.com/vulcanize/eth-pg-ipfs-state-snapshot/pkg/types"
)

var Block1_Header = types.Header{
	ParentHash:  common.HexToHash("0x6341fd3daf94b748c72ced5a5b26028f2474f5f00d824504e4fa37a75767e177"),
	UncleHash:   common.HexToHash("0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347"),
	Coinbase:    common.HexToAddress("0x0000000000000000000000000000000000000000"),
	Root:        common.HexToHash("0x53580584816f617295ea26c0e17641e0120cab2f0a8ffb53a866fd53aa8e8c2d"),
	TxHash:      common.HexToHash("0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421"),
	ReceiptHash: common.HexToHash("0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421"),
	Bloom:       types.Bloom{},
	Difficulty:  big.NewInt(+2),
	Number:      big.NewInt(+1),
	GasLimit:    4704588,
	GasUsed:     0,
	Time:        1492010458,
	Extra:       []byte{215, 131, 1, 6, 0, 132, 103, 101, 116, 104, 135, 103, 111, 49, 46, 55, 46, 51, 133, 108, 105, 110, 117, 120, 0, 0, 0, 0, 0, 0, 0, 0, 159, 30, 250, 30, 250, 114, 175, 19, 140, 145, 89, 102, 198, 57, 84, 74, 2, 85, 230, 40, 142, 24, 140, 34, 206, 145, 104, 193, 13, 190, 70, 218, 61, 136, 180, 170, 6, 89, 48, 17, 159, 184, 134, 33, 11, 240, 26, 8, 79, 222, 93, 59, 196, 141, 138, 163, 139, 202, 146, 228, 252, 197, 33, 81, 0},
	MixDigest:   common.Hash{},
	Nonce:       types.BlockNonce{},
	BaseFee:     nil,
}

var Block1_StateNode0 = snapt.Node{
	NodeType: 0,
	Path:     []byte{12, 0},
	Key:      common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000000"),
	Value:    []byte{248, 113, 160, 147, 141, 92, 6, 119, 63, 191, 125, 121, 193, 230, 153, 223, 49, 102, 109, 236, 50, 44, 161, 215, 28, 224, 171, 111, 118, 230, 79, 99, 18, 99, 4, 160, 117, 126, 95, 187, 60, 115, 90, 36, 51, 167, 59, 86, 20, 175, 63, 118, 94, 230, 107, 202, 41, 253, 234, 165, 214, 221, 181, 45, 9, 202, 244, 148, 128, 128, 128, 128, 128, 128, 128, 128, 128, 128, 128, 128, 128, 160, 247, 170, 155, 102, 71, 245, 140, 90, 255, 89, 193, 131, 99, 31, 85, 161, 78, 90, 0, 204, 46, 253, 15, 71, 120, 19, 109, 123, 255, 0, 188, 27, 128},
}
