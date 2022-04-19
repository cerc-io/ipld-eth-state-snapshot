package types

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type Publisher interface {
	PublishHeader(header *types.Header) error
	PublishStateNode(node *Node, headerID string, height uint64, tx Tx) error
	PublishStorageNode(node *Node, headerID string, height uint64, statePath []byte, tx Tx) error
	PublishCode(height uint64, codeHash common.Hash, codeBytes []byte, tx Tx) error
	BeginTx() (Tx, error)
	PrepareTxForBatch(tx Tx, batchSize uint) (Tx, error)
}

type Tx interface {
	Rollback() error
	Commit() error
}
