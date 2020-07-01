// Copyright © 2020 Vulcanize, Inc
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
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/multiformats/go-multihash"

	"github.com/vulcanize/ipfs-blockchain-watcher/pkg/eth"
	"github.com/vulcanize/ipfs-blockchain-watcher/pkg/ipfs/ipld"
	"github.com/vulcanize/ipfs-blockchain-watcher/pkg/postgres"
	"github.com/vulcanize/ipfs-blockchain-watcher/pkg/shared"
)

type Publisher struct {
	db *postgres.DB
}

func NewPublisher(db *postgres.DB) *Publisher {
	return &Publisher{
		db: db,
	}
}

func (p *Publisher) PublishHeader(header *types.Header) (int64, error) {
	headerNode, err := ipld.NewEthHeader(header)
	if err != nil {
		return 0, err
	}
	tx, err := p.db.Beginx()
	if err != nil {
		return 0, err
	}
	defer func() {
		if p := recover(); p != nil {
			shared.Rollback(tx)
			panic(p)
		} else if err != nil {
			shared.Rollback(tx)
		} else {
			err = tx.Commit()
		}
	}()
	if err := shared.PublishIPLD(tx, headerNode); err != nil {
		return 0, err
	}
	var headerID int64
	err = tx.QueryRowx(`INSERT INTO eth.header_cids (block_number, block_hash, parent_hash, cid, td, node_id, reward, state_root, tx_root, receipt_root, uncle_root, bloom, timestamp, times_validated)
 								VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
 								ON CONFLICT (block_number, block_hash) DO UPDATE SET block_number = header_cids.block_number
 								RETURNING id`,
		header.Number.Uint64(), header.Hash().Hex(), header.ParentHash.Hex(), headerNode.Cid(), "0", p.db.NodeID, "0", header.Root.Hex(), header.TxHash.Hex(),
		header.ReceiptHash.Hex(), header.UncleHash.Hex(), header.Bloom.Bytes(), header.Time, 0).Scan(&headerID)
	return headerID, err
}

func (p *Publisher) PublishStateNode(meta eth.StateNodeModel, nodeVal []byte, headerID int64) (int64, error) {
	var stateID int64
	var stateKey string
	if meta.StateKey != nullHash.String() {
		stateKey = meta.StateKey
	}
	tx, err := p.db.Beginx()
	if err != nil {
		return 0, err
	}
	defer func() {
		if p := recover(); p != nil {
			shared.Rollback(tx)
			panic(p)
		} else if err != nil {
			shared.Rollback(tx)
		} else {
			err = tx.Commit()
		}
	}()
	stateCIDStr, err := shared.PublishRaw(tx, ipld.MEthStateTrie, multihash.KECCAK_256, nodeVal)
	if err != nil {
		return 0, err
	}
	err = tx.QueryRowx(`INSERT INTO eth.state_cids (header_id, state_leaf_key, cid, state_path, node_type, diff) VALUES ($1, $2, $3, $4, $5, $6)
 									ON CONFLICT (header_id, state_path, diff) DO UPDATE SET (state_leaf_key, cid, node_type) = ($2, $3, $5, $6)
 									RETURNING id`,
		headerID, stateKey, stateCIDStr, meta.Path, meta.NodeType, false).Scan(&stateID)
	return stateID, err
}

func (p *Publisher) PublishStorageNode(meta eth.StorageNodeModel, nodeVal []byte, stateID int64) error {
	var storageKey string
	if meta.StorageKey != nullHash.String() {
		storageKey = meta.StorageKey
	}
	tx, err := p.db.Beginx()
	if err != nil {
		return err
	}
	defer func() {
		if p := recover(); p != nil {
			shared.Rollback(tx)
			panic(p)
		} else if err != nil {
			shared.Rollback(tx)
		} else {
			err = tx.Commit()
		}
	}()
	storageCIDStr, err := shared.PublishRaw(tx, ipld.MEthStorageTrie, multihash.KECCAK_256, nodeVal)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT INTO eth.storage_cids (state_id, storage_leaf_key, cid, storage_path, node_type, diff) VALUES ($1, $2, $3, $4, $5, $6) 
 							  ON CONFLICT (state_id, storage_path) DO UPDATE SET (storage_leaf_key, cid, node_type, diff) = ($2, $3, $5, $6)`,
		stateID, storageKey, storageCIDStr, meta.Path, meta.NodeType, false)
	return err
}
