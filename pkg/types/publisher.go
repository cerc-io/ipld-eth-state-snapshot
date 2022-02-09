package types

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type Publisher interface {
	PublishHeader(header *types.Header) error
	PublishStateNode(node *Node, headerID string, tx Tx) error
	PublishStorageNode(node *Node, headerID string, statePath []byte, tx Tx) error
	PublishCode(codeHash common.Hash, codeBytes []byte, tx Tx) error
	BeginTx() (Tx, error)
	PrepareTxForBatch(tx Tx, batchSize uint) (Tx, error)
}

type Tx interface {
	Rollback() error
	Commit() error
}
