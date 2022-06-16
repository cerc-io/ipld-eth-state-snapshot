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
	"strconv"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/statediff/indexer/database/sql"
	"github.com/ethereum/go-ethereum/statediff/indexer/database/sql/postgres"
	"github.com/ethereum/go-ethereum/statediff/indexer/ipld"
	"github.com/ethereum/go-ethereum/statediff/indexer/models"
	"github.com/multiformats/go-multihash"

	fixt "github.com/vulcanize/ipld-eth-state-snapshot/fixture"
	"github.com/vulcanize/ipld-eth-state-snapshot/pkg/snapshot/pg"
	snapt "github.com/vulcanize/ipld-eth-state-snapshot/pkg/types"
	"github.com/vulcanize/ipld-eth-state-snapshot/test"
)

var (
	pgConfig       = test.DefaultPgConfig
	nodeInfo       = test.DefaultNodeInfo
	snapshotHeight = 6

	pgQueryStateCids = `SELECT cast(state_cids.block_number AS TEXT), state_cids.cid, state_cids.state_leaf_key, state_cids.node_type, state_cids.state_path, state_cids.header_id, state_cids.mh_key
		FROM eth.state_cids
		WHERE eth.state_cids.block_number = $1
		ORDER BY state_cids.state_path`

	pgQueryStorageCids = `SELECT cast(storage_cids.block_number AS TEXT), storage_cids.cid, storage_cids.state_path, storage_cids.storage_leaf_key, storage_cids.node_type, storage_cids.storage_path, storage_cids.mh_key, storage_cids.header_id
		FROM eth.storage_cids
		WHERE eth.storage_cids.block_number = $1
		ORDER BY storage_cids.state_path, storage_cids.storage_path`

	pgIpfsGet = `SELECT data FROM public.blocks
		WHERE key = $1 AND block_number = $2`
)

func TestCreateInPlaceSnapshot(t *testing.T) {
	test.NeedsDB(t)
	ctx := context.Background()
	driver, err := postgres.NewSQLXDriver(ctx, pgConfig, nodeInfo)
	test.NoError(t, err)
	db := postgres.NewPostgresDB(driver)

	config := &Config{
		Eth: &EthConfig{
			NodeInfo: test.DefaultNodeInfo,
		},
		DB: &DBConfig{
			URI:        pgConfig.DbConnectionString(),
			ConnConfig: pgConfig,
		},
	}

	t.Run("Snapshot for blocks with contract deployment and transaction", func(t *testing.T) {
		sql.TearDownDB(t, db)
		_ = writeData(t, db, fixt.InPlaceSnapshotBlocks[:4])

		params := InPlaceSnapshotParams{StartHeight: uint64(0), EndHeight: uint64(snapshotHeight)}
		err = CreateInPlaceSnapshot(config, params)
		test.NoError(t, err)

		// Check inplace snapshot was created for state_cids
		compareStateNodes(t, db, fixt.ExpectedStateNodes)

		// Check inplace snapshot was created for storage_cids
		compareStorageNodes(t, db, fixt.ExpectedStorageNodes)
	})

	t.Run("Snapshot for blocks with contract destruction", func(t *testing.T) {
		t.Skip("Fix in-place snapshot function for removed type nodes")
		sql.TearDownDB(t, db)
		_ = writeData(t, db, fixt.InPlaceSnapshotBlocks[:5])

		params := InPlaceSnapshotParams{StartHeight: uint64(0), EndHeight: uint64(snapshotHeight)}
		err = CreateInPlaceSnapshot(config, params)
		test.NoError(t, err)

		// Check inplace snapshot was created for state_cids
		compareStateNodes(t, db, fixt.ExpectedStateNodesAfterContractDestruction)

		// Check inplace snapshot was created for storage_cids
		compareStorageNodes(t, db, fixt.ExpectedStorageNodesAfterContractDestruction)
	})

	t.Run("Snapshot with non-canonical blocks", func(t *testing.T) {
		t.Skip("Fix in-place snapshot function for chain with non-canonical blocks")
		sql.TearDownDB(t, db)
		_ = writeData(t, db, append(fixt.InPlaceSnapshotBlocks[:4], fixt.NonCanonicalChainFromBlock2...))

		params := InPlaceSnapshotParams{StartHeight: uint64(0), EndHeight: uint64(snapshotHeight)}
		err = CreateInPlaceSnapshot(config, params)
		test.NoError(t, err)

		// Check inplace snapshot was created for state_cids
		compareStateNodes(t, db, fixt.ExpectedStateNodes)

		// Check inplace snapshot was created for storage_cids
		compareStorageNodes(t, db, fixt.ExpectedStorageNodes)
	})

	t.Run("Snapshot for chain with non indexed missing blocks", func(t *testing.T) {
		t.Skip("Fix in-place snapshot function for chain with non-indexed missing blocks")
		sql.TearDownDB(t, db)
		_ = writeData(t, db, fixt.ChainWithMissingBlock)

		params := InPlaceSnapshotParams{StartHeight: uint64(0), EndHeight: uint64(snapshotHeight)}
		err = CreateInPlaceSnapshot(config, params)
		test.NoError(t, err)

		// Check inplace snapshot was created for state_cids
		compareStateNodes(t, db, fixt.ExpectedStateNodes)

		// Check inplace snapshot was created for storage_cids
		compareStorageNodes(t, db, fixt.ExpectedStorageNodes)
	})
}

func writeData(t *testing.T, db *postgres.DB, blocks []fixt.Block) snapt.Publisher {
	pub := pg.NewPublisher(db)
	tx, err := pub.BeginTx()
	test.NoError(t, err)

	for _, block := range blocks {
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

	test.NoError(t, pub.PublishHeader(&fixt.Block6_Header))
	return pub
}

func compareStateNodes(t *testing.T, db *postgres.DB, expectedStateNodes []snapt.Node) {
	ctx := context.Background()
	stateNodes := make([]models.StateNodeModel, 0)
	err := db.Select(ctx, &stateNodes, pgQueryStateCids, snapshotHeight)
	test.NoError(t, err)
	test.ExpectEqual(t, len(expectedStateNodes), len(stateNodes))

	for index, stateNode := range stateNodes {
		var data []byte
		err = db.Get(ctx, &data, pgIpfsGet, stateNode.MhKey, snapshotHeight)
		test.NoError(t, err)

		expectedStateNode := expectedStateNodes[index]
		expectedCID, _ := ipld.RawdataToCid(ipld.MEthStateTrie, expectedStateNode.Value, multihash.KECCAK_256)
		test.ExpectEqual(t, strconv.Itoa(snapshotHeight), stateNode.BlockNumber)
		test.ExpectEqual(t, fixt.Block6_Header.Hash().String(), stateNode.HeaderID)
		test.ExpectEqual(t, expectedCID.String(), stateNode.CID)
		test.ExpectEqual(t, int(expectedStateNode.NodeType), stateNode.NodeType)
		test.ExpectEqual(t, expectedStateNode.Key, common.HexToHash(stateNode.StateKey))
		test.ExpectEqual(t, false, stateNode.Diff)
		test.ExpectEqualBytes(t, expectedStateNode.Path, stateNode.Path)
		test.ExpectEqualBytes(t, expectedStateNode.Value, data)
	}
}

func compareStorageNodes(t *testing.T, db *postgres.DB, expectedStorageNodes []fixt.StorageNodeWithState) {
	ctx := context.Background()
	storageNodes := make([]models.StorageNodeModel, 0)
	err := db.Select(ctx, &storageNodes, pgQueryStorageCids, snapshotHeight)
	test.NoError(t, err)
	test.ExpectEqual(t, len(expectedStorageNodes), len(storageNodes))

	for index, storageNode := range storageNodes {
		expectedStorageNode := expectedStorageNodes[index]
		expectedStorageCID, _ := ipld.RawdataToCid(ipld.MEthStorageTrie, expectedStorageNode.Value, multihash.KECCAK_256)

		test.ExpectEqual(t, strconv.Itoa(snapshotHeight), storageNode.BlockNumber)
		test.ExpectEqual(t, fixt.Block6_Header.Hash().String(), storageNode.HeaderID)
		test.ExpectEqual(t, expectedStorageCID.String(), storageNode.CID)
		test.ExpectEqual(t, int(expectedStorageNode.NodeType), storageNode.NodeType)
		test.ExpectEqual(t, expectedStorageNode.Key, common.HexToHash(storageNode.StorageKey))
		test.ExpectEqual(t, expectedStorageNode.StatePath, storageNode.StatePath)
		test.ExpectEqual(t, expectedStorageNode.Path, storageNode.Path)
		test.ExpectEqual(t, false, storageNode.Diff)

		var data []byte
		err = db.Get(ctx, &data, pgIpfsGet, storageNode.MhKey, snapshotHeight)
		test.NoError(t, err)
		test.ExpectEqualBytes(t, expectedStorageNode.Value, data)
	}
}
