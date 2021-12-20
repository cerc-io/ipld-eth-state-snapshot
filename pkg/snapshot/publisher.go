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
	"bytes"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/multiformats/go-multihash"

	"github.com/ethereum/go-ethereum/statediff/indexer/ipfs/ipld"
	"github.com/ethereum/go-ethereum/statediff/indexer/postgres"
	"github.com/ethereum/go-ethereum/statediff/indexer/shared"
)

// Publisher is wrapper around DB.
type Publisher struct {
	db            *postgres.DB
}

// NewPublisher creates Publisher
func NewPublisher(db *postgres.DB) *Publisher {
	return &Publisher{
		db:            db,
	}
}

// PublishHeader writes the header to the ipfs backing pg datastore and adds secondary indexes in the header_cids table
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

	if err = shared.PublishIPLD(tx, headerNode); err != nil {
		return 0, err
	}

	mhKey, _ := shared.MultihashKeyFromCIDString(headerNode.Cid().String())
	var headerID int64
	err = tx.QueryRowx(`INSERT INTO eth.header_cids (block_number, block_hash, parent_hash, cid, td, node_id, reward, state_root, tx_root, receipt_root, uncle_root, bloom, timestamp, mh_key, times_validated)
 								VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
 								ON CONFLICT (block_number, block_hash) DO UPDATE SET block_number = header_cids.block_number
 								RETURNING id`,
		header.Number.Uint64(), header.Hash().Hex(), header.ParentHash.Hex(), headerNode.Cid().String(), "0", p.db.NodeID, "0", header.Root.Hex(), header.TxHash.Hex(),
		header.ReceiptHash.Hex(), header.UncleHash.Hex(), header.Bloom.Bytes(), header.Time, mhKey, 0).Scan(&headerID)

	return headerID, err
}

// PublishStateNode writes the state node to the ipfs backing datastore and adds secondary indexes in the state_cids table
func (p *Publisher) PublishStateNode(node *node, headerID int64) (int64, error) {
	var stateID int64
	var stateKey string
	if !bytes.Equal(node.key.Bytes(), nullHash.Bytes()) {
		stateKey = node.key.Hex()
	}

	tx, err := p.db.Beginx()
	if err != nil {
		return 0, err
	}

	defer func() {
		if rec := recover(); rec != nil {
			shared.Rollback(tx)
			panic(rec)
		} else if err != nil {
			shared.Rollback(tx)
		} else {
			err = tx.Commit()
		}
	}()

	stateCIDStr, mhKey, err := shared.PublishRaw(tx, ipld.MEthStateTrie, multihash.KECCAK_256, node.value)
	if err != nil {
		return 0, err
	}

	err = tx.QueryRowx(`INSERT INTO eth.state_cids (header_id, state_leaf_key, cid, state_path, node_type, diff, mh_key) VALUES ($1, $2, $3, $4, $5, $6, $7)
 									ON CONFLICT (header_id, state_path) DO UPDATE SET (state_leaf_key, cid, node_type, diff, mh_key) = ($2, $3, $5, $6, $7)
 									RETURNING id`,
		headerID, stateKey, stateCIDStr, node.path, node.nodeType, false, mhKey).Scan(&stateID)

	return stateID, err
}

// PublishStorageNode writes the storage node to the ipfs backing pg datastore and adds secondary indexes in the storage_cids table
func (p *Publisher) PublishStorageNode(node *node, stateID int64) error {
	var storageKey string
	if !bytes.Equal(node.key.Bytes(), nullHash.Bytes()) {
		storageKey = node.key.Hex()
	}

	tx, err := p.db.Beginx()
	if err != nil {
		return err
	}

	defer func() {
		if rec := recover(); rec != nil {
			shared.Rollback(tx)
			panic(rec)
		} else if err != nil {
			shared.Rollback(tx)
		} else {
			err = tx.Commit()
		}
	}()

	storageCIDStr, mhKey, err := shared.PublishRaw(tx, ipld.MEthStorageTrie, multihash.KECCAK_256, node.value)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`INSERT INTO eth.storage_cids (state_id, storage_leaf_key, cid, storage_path, node_type, diff, mh_key) VALUES ($1, $2, $3, $4, $5, $6, $7)
                              	ON CONFLICT (state_id, storage_path) DO UPDATE SET (storage_leaf_key, cid, node_type, diff, mh_key) = ($2, $3, $5, $6, $7)`,
		stateID, storageKey, storageCIDStr, node.path, node.nodeType, false, mhKey)
	if err != nil {
		return err
	}

	return nil
}

// PublishCode writes code to the ipfs backing pg datastore
func (p *Publisher) PublishCode(codeHash common.Hash, codeBytes []byte) error {
	// no codec for code, doesn't matter though since blockstore key is multihash-derived
	mhKey, err := shared.MultihashKeyFromKeccak256(codeHash)
	if err != nil {
		return fmt.Errorf("error deriving multihash key from codehash: %v", err)
	}

	tx, err := p.db.Beginx()
	if err != nil {
		return err
	}

	defer func() {
		if rec := recover(); rec != nil {
			shared.Rollback(tx)
			panic(rec)
		} else if err != nil {
			shared.Rollback(tx)
		} else {
			err = tx.Commit()
		}
	}()

	if err = shared.PublishDirect(tx, mhKey, codeBytes); err != nil {
		return fmt.Errorf("error publishing code IPLD: %v", err)
	}

	return nil
}