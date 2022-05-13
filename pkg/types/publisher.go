package types

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type Publisher interface {
	PublishHeader(header *types.Header) error
	PublishStateNode(node *Node, headerID string, height *big.Int, tx Tx) error
	PublishStorageNode(node *Node, headerID string, height *big.Int, statePath []byte, tx Tx) error
	PublishCode(height *big.Int, codeHash common.Hash, codeBytes []byte, tx Tx) error
	BeginTx() (Tx, error)
	PrepareTxForBatch(tx Tx, batchSize uint) (Tx, error)
}

type Tx interface {
	Rollback() error
	Commit() error
}
