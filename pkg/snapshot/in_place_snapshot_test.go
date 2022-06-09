package snapshot

import (
	"context"
	"fmt"
	"testing"

	"github.com/ethereum/go-ethereum/statediff/indexer/database/sql/postgres"
	"github.com/ethereum/go-ethereum/statediff/indexer/models"
	"github.com/jmoiron/sqlx"

	fixt "github.com/vulcanize/ipld-eth-state-snapshot/fixture"
	"github.com/vulcanize/ipld-eth-state-snapshot/pkg/snapshot/pg"
	snapt "github.com/vulcanize/ipld-eth-state-snapshot/pkg/types"
	"github.com/vulcanize/ipld-eth-state-snapshot/test"
)

var (
	pgConfig = postgres.Config{
		Hostname:     "localhost",
		Port:         8077,
		DatabaseName: "vulcanize_testing",
		Username:     "vdbm",
		Password:     "password",

		MaxIdle:         0,
		MaxConnLifetime: 0,
		MaxConns:        4,
	}
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

func writeData(t *testing.T) snapt.Publisher {
	driver, err := postgres.NewPGXDriver(context.Background(), pgConfig, nodeInfo)
	test.NoError(t, err)
	pub := pg.NewPublisher(postgres.NewPostgresDB(driver))
	tx, err := pub.BeginTx()
	test.NoError(t, err)

	for _, block := range fixt.InPlaceBlocks {
		headerID := block.Hash.String()

		for _, stateNode := range block.StateNodes {
			test.NoError(t, pub.PublishStateNode(&stateNode, headerID, block.Number, tx))
		}

		for index, stateStorageNodes := range block.StorageNodes {
			stateNode := block.StateNodes[index]

			for _, storageNode := range stateStorageNodes {
				test.NoError(t, pub.PublishStorageNode(&storageNode, headerID, block.Number, stateNode.Path, tx))
			}
		}

	}

	test.NoError(t, pub.PublishHeader(&fixt.Block5_Header))

	test.NoError(t, tx.Commit())
	return pub
}

func TestCreateInPlaceSnapshot(t *testing.T) {
	test.NeedsDB(t)

	ctx := context.Background()
	db, err := sqlx.ConnectContext(ctx, "postgres", pgConfig.DbConnectionString())
	test.NoError(t, err)

	// clear existing test data
	pgDeleteTable := `DELETE FROM %s`
	for _, tbl := range allTables {
		_, err = db.Exec(fmt.Sprintf(pgDeleteTable, tbl.Name))
		test.NoError(t, err)
	}

	_ = writeData(t)

	params := InPlaceSnapshotParams{StartHeight: uint64(0), EndHeight: uint64(5)}
	config := &Config{
		Eth: &EthConfig{
			NodeInfo: test.DefaultNodeInfo,
		},
		DB: &DBConfig{
			URI:        pgConfig.DbConnectionString(),
			ConnConfig: pgConfig,
		},
	}
	err = CreateInPlaceSnapshot(config, params)
	test.NoError(t, err)

	// check inplace snapshot was created for state_cids
	stateNodes := make([]models.StateNodeModel, 0)
	pgQueryStates := `SELECT state_cids.cid, state_cids.state_leaf_key, state_cids.node_type, state_cids.state_path, state_cids.header_id
					  FROM eth.state_cids
					  WHERE eth.state_cids.block_number = $1`

	err = db.Select(&stateNodes, pgQueryStates, 5)
	test.NoError(t, err)
	test.ExpectEqual(t, 4, len(stateNodes))

	// TODO: Compare stateNodes expected fields
}
