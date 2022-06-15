package pg

import (
	"context"
	"testing"

	"github.com/ethereum/go-ethereum/statediff/indexer/database/sql"
	"github.com/ethereum/go-ethereum/statediff/indexer/database/sql/postgres"
	"github.com/ethereum/go-ethereum/statediff/indexer/ipld"

	fixt "github.com/vulcanize/ipld-eth-state-snapshot/fixture"
	snapt "github.com/vulcanize/ipld-eth-state-snapshot/pkg/types"
	"github.com/vulcanize/ipld-eth-state-snapshot/test"
)

var (
	pgConfig = test.DefaultPgConfig
	nodeInfo = test.DefaultNodeInfo
	// tables ordered according to fkey depedencies
	allTables = []*snapt.Table{
		&snapt.TableIPLDBlock,
		&snapt.TableNodeInfo,
		&snapt.TableHeader,
		&snapt.TableStateNode,
		&snapt.TableStorageNode,
	}
)

func writeData(t *testing.T, db *postgres.DB) *publisher {
	pub := NewPublisher(db)
	test.NoError(t, pub.PublishHeader(&fixt.Block1_Header))
	tx, err := pub.BeginTx()
	test.NoError(t, err)

	headerID := fixt.Block1_Header.Hash().String()
	test.NoError(t, pub.PublishStateNode(&fixt.Block1_StateNode0, headerID, fixt.Block1_Header.Number, tx))

	test.NoError(t, tx.Commit())
	return pub
}

// Note: DB user requires role membership "pg_read_server_files"
func TestBasic(t *testing.T) {
	test.NeedsDB(t)

	ctx := context.Background()
	driver, err := postgres.NewSQLXDriver(ctx, pgConfig, nodeInfo)
	test.NoError(t, err)
	db := postgres.NewPostgresDB(driver)

	sql.TearDownDB(t, db)

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
