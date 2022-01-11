package types

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/jmoiron/sqlx"
)

type Publisher interface {
	PublishHeader(header *types.Header) (int64, error)
	PublishStateNode(node *Node, headerID int64, tx *sqlx.Tx) (int64, error)
	PublishStorageNode(node *Node, stateID int64, tx *sqlx.Tx) error
	PublishCode(codeHash common.Hash, codeBytes []byte, tx *sqlx.Tx) error
	BeginTx() (*sqlx.Tx, error)
	CommitTx(*sqlx.Tx) error
	PrepareTxForBatch(tx *sqlx.Tx, batchSize uint) (*sqlx.Tx, error)
}
