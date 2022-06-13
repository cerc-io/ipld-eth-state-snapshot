// Copyright © 2022 Vulcanize, Inc
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

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

	for _, block := range fixt.InPlaceSnapshotBlocks[0:snapshotHeight] {
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

	test.NoError(t, tx.Commit())

	test.NoError(t, pub.PublishHeader(&fixt.Block4_Header))
	return pub
}

func TestCreateInPlaceSnapshot(t *testing.T) {
	test.NeedsDB(t)
	ctx := context.Background()
	db, err := sqlx.ConnectContext(ctx, "postgres", pgConfig.DbConnectionString())
	test.NoError(t, err)

	// Clear existing test data
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

	// Check inplace snapshot was created for state_cids
	stateNodes := make([]models.StateNodeModel, 0)
	pgQueryStateCids := `SELECT cast(state_cids.block_number AS TEXT), state_cids.cid, state_cids.state_leaf_key, state_cids.node_type, state_cids.state_path, state_cids.header_id, state_cids.mh_key
					  FROM eth.state_cids
					  WHERE eth.state_cids.block_number = $1
					  ORDER BY state_cids.state_path`

	err = db.Select(&stateNodes, pgQueryStateCids, snapshotHeight)
	test.NoError(t, err)
	test.ExpectEqual(t, 4, len(stateNodes))
	expectedStateNodes := fixt.InPlaceSnapshotBlocks[snapshotHeight].StateNodes

	pgIpfsGet := `SELECT data FROM public.blocks
					WHERE key = $1 AND block_number = $2`

	for index, stateNode := range stateNodes {
		var data []byte
		err = db.Get(&data, pgIpfsGet, stateNode.MhKey, snapshotHeight)
		test.NoError(t, err)

		expectedStateNode := expectedStateNodes[index]
		expectedCID, _ := ipld.RawdataToCid(ipld.MEthStateTrie, expectedStateNode.Value, multihash.KECCAK_256)
		test.ExpectEqual(t, strconv.Itoa(snapshotHeight), stateNode.BlockNumber)
		test.ExpectEqual(t, fixt.Block4_Header.Hash().String(), stateNode.HeaderID)
		test.ExpectEqual(t, expectedCID.String(), stateNode.CID)
		test.ExpectEqual(t, int(expectedStateNode.NodeType), stateNode.NodeType)
		test.ExpectEqual(t, expectedStateNode.Key, common.HexToHash(stateNode.StateKey))
		test.ExpectEqual(t, false, stateNode.Diff)
		test.ExpectEqualBytes(t, expectedStateNode.Path, stateNode.Path)
		test.ExpectEqualBytes(t, expectedStateNode.Value, data)
	}

	// Check inplace snapshot was created for storage_cids
	storageNodes := make([]models.StorageNodeModel, 0)
	pgQueryStorageCids := `SELECT cast(storage_cids.block_number AS TEXT), storage_cids.cid, storage_cids.state_path, storage_cids.storage_leaf_key, storage_cids.node_type, storage_cids.storage_path, storage_cids.mh_key, storage_cids.header_id
					  FROM eth.storage_cids
					  WHERE eth.storage_cids.block_number = $1
					  ORDER BY storage_cids.storage_path`
	err = db.Select(&storageNodes, pgQueryStorageCids, snapshotHeight)
	test.NoError(t, err)
	test.ExpectEqual(t, 1, len(storageNodes))
	expectedStorageNode := fixt.InPlaceSnapshotBlocks[snapshotHeight].StorageNodes[0][0]
	expectedStorageCID, _ := ipld.RawdataToCid(ipld.MEthStorageTrie, expectedStorageNode.Value, multihash.KECCAK_256)

	test.ExpectEqual(t, strconv.Itoa(snapshotHeight), storageNodes[0].BlockNumber)
	test.ExpectEqual(t, fixt.Block4_Header.Hash().String(), storageNodes[0].HeaderID)
	test.ExpectEqual(t, expectedStorageCID.String(), storageNodes[0].CID)
	test.ExpectEqual(t, int(expectedStorageNode.NodeType), storageNodes[0].NodeType)
	test.ExpectEqual(t, expectedStorageNode.Key.Hex(), storageNodes[0].StorageKey)
	test.ExpectEqual(t, fixt.InPlaceSnapshotBlocks[snapshotHeight].StateNodes[2].Path, storageNodes[0].StatePath)
	test.ExpectEqual(t, expectedStorageNode.Path, storageNodes[0].Path)
	test.ExpectEqual(t, false, storageNodes[0].Diff)

	var data []byte
	err = db.Get(&data, pgIpfsGet, storageNodes[0].MhKey, snapshotHeight)
	test.NoError(t, err)
	test.ExpectEqualBytes(t, expectedStorageNode.Value, data)
}
