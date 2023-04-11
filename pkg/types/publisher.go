package types

import (
	"math/big"

	"github.com/ethereum/go-ethereum/statediff/indexer/models"
	"github.com/ipfs/go-cid"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type Publisher interface {
	PublishHeader(header *types.Header) error
	PublishStateLeafNode(node *models.StateNodeModel, tx Tx) error
	PublishStorageLeafNode(node *models.StorageNodeModel, tx Tx) error
	PublishCode(height *big.Int, codeHash common.Hash, codeBytes []byte, tx Tx) error
	PublishIPLD(c cid.Cid, raw []byte, height *big.Int, tx Tx) (string, error)
	BeginTx() (Tx, error)
	PrepareTxForBatch(tx Tx, batchSize uint) (Tx, error)
}

type Tx interface {
	Rollback() error
	Commit() error
}
