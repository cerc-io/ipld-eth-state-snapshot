package pg

import (
	"context"
	"fmt"
	"testing"

	"github.com/ethereum/go-ethereum/statediff/indexer/database/sql/postgres"
	"github.com/ethereum/go-ethereum/statediff/indexer/ipld"
	"github.com/jackc/pgx/v4"

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

func writeData(t *testing.T) *publisher {
	driver, err := postgres.NewPGXDriver(context.Background(), pgConfig, nodeInfo)
	test.NoError(t, err)
	pub := NewPublisher(postgres.NewPostgresDB(driver))
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
	conn, err := pgx.Connect(ctx, pgConfig.DbConnectionString())
	test.NoError(t, err)

	// clear existing test data
	pgDeleteTable := `DELETE FROM %s`
	for _, tbl := range allTables {
		_, err = conn.Exec(ctx, fmt.Sprintf(pgDeleteTable, tbl.Name))
		test.NoError(t, err)
	}

	_ = writeData(t)

	// check header was successfully committed
	pgQueryHeader := `SELECT cid, block_hash
					  FROM eth.header_cids
				      WHERE block_number = $1`
	type res struct {
		CID       string
		BlockHash string
	}
	var header res
	err = conn.QueryRow(ctx, pgQueryHeader, fixt.Block1_Header.Number.Uint64()).Scan(
		&header.CID, &header.BlockHash)
	test.NoError(t, err)

	headerNode, err := ipld.NewEthHeader(&fixt.Block1_Header)
	test.NoError(t, err)
	test.ExpectEqual(t, headerNode.Cid().String(), header.CID)
	test.ExpectEqual(t, fixt.Block1_Header.Hash().String(), header.BlockHash)
}
