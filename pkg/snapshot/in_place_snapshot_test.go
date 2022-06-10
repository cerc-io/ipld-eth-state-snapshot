package snapshot

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/statediff/indexer/database/sql/postgres"
	"github.com/ethereum/go-ethereum/statediff/indexer/ipld"
	"github.com/ethereum/go-ethereum/statediff/indexer/models"
	"github.com/jmoiron/sqlx"
	"github.com/multiformats/go-multihash"

	fixt "github.com/vulcanize/ipld-eth-state-snapshot/fixture"
	"github.com/vulcanize/ipld-eth-state-snapshot/pkg/snapshot/pg"
	snapt "github.com/vulcanize/ipld-eth-state-snapshot/pkg/types"
	"github.com/vulcanize/ipld-eth-state-snapshot/test"
)

var (
	pgConfig       = test.DefaultPgConfig
	nodeInfo       = test.DefaultNodeInfo
	snapshotHeight = 4

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

	for _, block := range fixt.InPlaceBlocks[0:snapshotHeight] {
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

	params := InPlaceSnapshotParams{StartHeight: uint64(0), EndHeight: uint64(snapshotHeight)}
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
	pgQueryStateCids := `SELECT state_cids.cid, state_cids.state_leaf_key, state_cids.node_type, state_cids.state_path, state_cids.header_id, state_cids.mh_key
					  FROM eth.state_cids
					  WHERE eth.state_cids.block_number = $1`

	err = db.Select(&stateNodes, pgQueryStateCids, snapshotHeight)
	test.NoError(t, err)
	test.ExpectEqual(t, 4, len(stateNodes))
	expectedStateNodes := fixt.InPlaceBlocks[snapshotHeight].StateNodes

	pgIpfsGet := `SELECT data FROM public.blocks
					WHERE key = $1 AND block_number = $2`

	for index, stateNode := range stateNodes {
		var data []byte
		err = db.Get(&data, pgIpfsGet, stateNode.MhKey, snapshotHeight)
		test.NoError(t, err)

		expectedStateNode := expectedStateNodes[index]
		expectedCID, _ := ipld.RawdataToCid(ipld.MEthStateTrie, expectedStateNode.Value, multihash.KECCAK_256)
		test.ExpectEqual(t, expectedCID.String(), stateNode.CID)
		test.ExpectEqual(t, int(expectedStateNode.NodeType), stateNode.NodeType)
		test.ExpectEqual(t, expectedStateNode.Key, common.HexToHash(stateNode.StateKey))
		test.ExpectEqualBytes(t, expectedStateNode.Path, stateNode.Path)
		test.ExpectEqualBytes(t, expectedStateNode.Value, data)
	}

	// check inplace snapshot was created for state_cids
	storageNodes := make([]models.StorageNodeModel, 0)
	pgQueryStorageCids := `SELECT cast(storage_cids.block_number AS TEXT), storage_cids.cid, storage_cids.state_path, storage_cids.storage_leaf_key, storage_cids.node_type, storage_cids.storage_path
					  FROM eth.storage_cids
					  WHERE eth.storage_cids.block_number = $1`
	err = db.Select(&storageNodes, pgQueryStorageCids, snapshotHeight)
	test.NoError(t, err)
	test.ExpectEqual(t, 1, len(storageNodes))
	expectedStorageNode := fixt.InPlaceBlocks[snapshotHeight].StorageNodes[0][0]
	expectedStorageCID, _ := ipld.RawdataToCid(ipld.MEthStorageTrie, expectedStorageNode.Value, multihash.KECCAK_256)

	test.ExpectEqual(t, models.StorageNodeModel{
		BlockNumber: strconv.Itoa(snapshotHeight),
		CID:         expectedStorageCID.String(),
		NodeType:    2,
		StorageKey:  expectedStorageNode.Key.Hex(),
		StatePath:   fixt.InPlaceBlocks[snapshotHeight].StateNodes[2].Path,
		Path:        expectedStorageNode.Path,
	}, storageNodes[0])
}
