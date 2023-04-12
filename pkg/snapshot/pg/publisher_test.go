package pg

import (
	"context"
	"testing"

	"github.com/ethereum/go-ethereum/statediff/indexer/shared/schema"

	"github.com/ethereum/go-ethereum/statediff/indexer/database/sql/postgres"
	"github.com/ethereum/go-ethereum/statediff/indexer/ipld"
	"github.com/ethereum/go-ethereum/statediff/indexer/test_helpers"

	fixt "github.com/cerc-io/ipld-eth-state-snapshot/fixture"
	"github.com/cerc-io/ipld-eth-state-snapshot/test"
)

var (
	pgConfig = test.DefaultPgConfig
	nodeInfo = test.DefaultNodeInfo
	// tables ordered according to fkey depedencies
	allTables = []*schema.Table{
		&schema.TableIPLDBlock,
		&schema.TableNodeInfo,
		&schema.TableHeader,
		&schema.TableStateNode,
		&schema.TableStorageNode,
	}
)

func writeData(t *testing.T, db *postgres.DB) *publisher {
	pub := NewPublisher(db)
	test.NoError(t, pub.PublishHeader(&fixt.Block1_Header))
	tx, err := pub.BeginTx()
	test.NoError(t, err)

	headerID := fixt.Block1_Header.Hash().String()
	stateNode := &fixt.Block1_StateNode0
	test.NoError(t, pub.PublishStateLeafNode(&fixt.Block1_StateNode0, headerID, fixt.Block1_Header.Number, tx))

	test.NoError(t, tx.Commit())
	return pub
}

// Note: DB user requires role membership "pg_read_server_files"
func TestBasic(t *testing.T) {
	test.NeedsDB(t)

	ctx := context.Background()
	driver, err := postgres.NewSQLXDriver(ctx, pgConfig, nodeInfo)
	test.NoError(t, err)
	db := postgres.NewPostgresDB(driver, false)

	test_helpers.TearDownDB(t, db)

	_ = writeData(t, db)

	// check header was successfully committed
	pgQueryHeader := `SELECT cid, block_hash
					  FROM eth.header_cids
				      WHERE block_number = $1`
	type res struct {
		CID       string
		BlockHash string
	}
	var header res
	err = db.QueryRow(ctx, pgQueryHeader, fixt.Block1_Header.Number.Uint64()).Scan(
		&header.CID, &header.BlockHash)
	test.NoError(t, err)

	headerNode, err := ipld.NewEthHeader(&fixt.Block1_Header)
	test.NoError(t, err)
	test.ExpectEqual(t, headerNode.Cid().String(), header.CID)
	test.ExpectEqual(t, fixt.Block1_Header.Hash().String(), header.BlockHash)
}
