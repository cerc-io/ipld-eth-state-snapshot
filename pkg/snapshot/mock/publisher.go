package mock

import (
	"fmt"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/jmoiron/sqlx"

	"github.com/golang/mock/gomock"

	mocks "github.com/vulcanize/eth-pg-ipfs-state-snapshot/mocks/snapshot"
	snapt "github.com/vulcanize/eth-pg-ipfs-state-snapshot/pkg/types"
)

type MockPublisher struct {
	*mocks.MockPublisher
}

func NewMockPublisher(t *testing.T) *MockPublisher {
	ctl := gomock.NewController(t)
	return &MockPublisher{mocks.NewMockPublisher(ctl)}
}

func dump(funcname string, xs ...interface{}) {
	if true {
		return
	}
	fmt.Printf(">> %s", funcname)
	fmt.Printf(strings.Repeat(" %+v", len(xs))+"\n", xs...)
}

func (p *MockPublisher) PublishHeader(header *types.Header) (int64, error) {
	// fmt.Printf("PublishHeader %+v\n", header)
	dump("PublishHeader", header)
	return p.MockPublisher.PublishHeader(header)
}
func (p *MockPublisher) PublishStateNode(node *snapt.Node, headerID int64, tx *sqlx.Tx) (int64, error) {
	dump("PublishStateNode", node, headerID)
	return p.MockPublisher.PublishStateNode(node, headerID, tx)

}
func (p *MockPublisher) PublishStorageNode(node *snapt.Node, stateID int64, tx *sqlx.Tx) error {
	dump("PublishStorageNode", node, stateID)
	return p.MockPublisher.PublishStorageNode(node, stateID, tx)
}
func (p *MockPublisher) PublishCode(codeHash common.Hash, codeBytes []byte, tx *sqlx.Tx) error {
	dump("PublishCode", codeHash, codeBytes)
	return p.MockPublisher.PublishCode(codeHash, codeBytes, tx)
}
func (p *MockPublisher) BeginTx() (*sqlx.Tx, error) {
	dump("BeginTx")
	return p.MockPublisher.BeginTx()
}
func (p *MockPublisher) CommitTx(tx *sqlx.Tx) error {
	dump("CommitTx", tx)
	return p.MockPublisher.CommitTx(tx)
}
func (p *MockPublisher) PrepareTxForBatch(tx *sqlx.Tx, batchSize uint) (*sqlx.Tx, error) {
	dump("PrepareTxForBatch", tx, batchSize)
	return p.MockPublisher.PrepareTxForBatch(tx, batchSize)
}
