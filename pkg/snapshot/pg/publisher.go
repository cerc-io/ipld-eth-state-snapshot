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

package pg

import (
	"context"
	"fmt"
	"math/big"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ipfs/go-cid"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	dshelp "github.com/ipfs/go-ipfs-ds-help"
	"github.com/multiformats/go-multihash"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"

	"github.com/ethereum/go-ethereum/statediff/indexer/database/sql"
	"github.com/ethereum/go-ethereum/statediff/indexer/database/sql/postgres"
	"github.com/ethereum/go-ethereum/statediff/indexer/ipld"
	"github.com/ethereum/go-ethereum/statediff/indexer/shared"
	"github.com/vulcanize/ipld-eth-state-snapshot/pkg/prom"
	snapt "github.com/vulcanize/ipld-eth-state-snapshot/pkg/types"
)

var _ snapt.Publisher = (*publisher)(nil)

const logInterval = 1 * time.Minute

// Publisher is wrapper around DB.
type publisher struct {
	db                 *postgres.DB
	currBatchSize      uint
	stateNodeCounter   uint64
	storageNodeCounter uint64
	codeNodeCounter    uint64
	startTime          time.Time
}

// NewPublisher creates Publisher
func NewPublisher(db *postgres.DB) *publisher {
	return &publisher{
		db:        db,
		startTime: time.Now(),
	}
}

type pubTx struct {
	sql.Tx
	callback func()
}

func (tx pubTx) Rollback() error { return tx.Tx.Rollback(context.Background()) }
func (tx pubTx) Commit() error {
	if tx.callback != nil {
		defer tx.callback()
	}
	return tx.Tx.Commit(context.Background())
}
func (tx pubTx) Exec(sql string, args ...interface{}) (sql.Result, error) {
	return tx.Tx.Exec(context.Background(), sql, args...)
}

func (p *publisher) BeginTx() (snapt.Tx, error) {
	tx, err := p.db.Begin(context.Background())
	if err != nil {
		return nil, err
	}
	go p.logNodeCounters()
	return pubTx{tx, func() {
		p.printNodeCounters("final stats")
	}}, nil
}

// PublishRaw derives a cid from raw bytes and provided codec and multihash type, and writes it to the db tx
// returns the CID and blockstore prefixed multihash key
func (tx pubTx) publishRaw(codec uint64, raw []byte, height *big.Int) (cid, prefixedKey string, err error) {
	c, err := ipld.RawdataToCid(codec, raw, multihash.KECCAK_256)
	if err != nil {
		return
	}
	cid = c.String()
	prefixedKey, err = tx.publishIPLD(c, raw, height)
	return
}

func (tx pubTx) publishIPLD(c cid.Cid, raw []byte, height *big.Int) (string, error) {
	dbKey := dshelp.MultihashToDsKey(c.Hash())
	prefixedKey := blockstore.BlockPrefix.String() + dbKey.String()
	_, err := tx.Exec(snapt.TableIPLDBlock.ToInsertStatement(), height.Uint64(), prefixedKey, raw)
	return prefixedKey, err
}

// PublishHeader writes the header to the ipfs backing pg datastore and adds secondary indexes in the header_cids table
func (p *publisher) PublishHeader(header *types.Header) (err error) {
	headerNode, err := ipld.NewEthHeader(header)
	if err != nil {
		return err
	}

	snapTx, err := p.db.Begin(context.Background())
	if err != nil {
		return err
	}
	tx := pubTx{snapTx, nil}
	defer func() {
		err = snapt.CommitOrRollback(tx, err)
		if err != nil {
			logrus.Errorf("CommitOrRollback failed: %s", err)
		}
	}()

	if _, err = tx.publishIPLD(headerNode.Cid(), headerNode.RawData(), header.Number); err != nil {
		return err
	}

	mhKey := shared.MultihashKeyFromCID(headerNode.Cid())
	_, err = tx.Exec(snapt.TableHeader.ToInsertStatement(), header.Number.Uint64(), header.Hash().Hex(),
		header.ParentHash.Hex(), headerNode.Cid().String(), "0", p.db.NodeID(), "0",
		header.Root.Hex(), header.TxHash.Hex(), header.ReceiptHash.Hex(), header.UncleHash.Hex(),
		header.Bloom.Bytes(), header.Time, mhKey, 0, header.Coinbase.String())
	return err
}

// PublishStateNode writes the state node to the ipfs backing datastore and adds secondary indexes in the state_cids table
func (p *publisher) PublishStateNode(node *snapt.Node, headerID string, height *big.Int, snapTx snapt.Tx) error {
	var stateKey string
	if !snapt.IsNullHash(node.Key) {
		stateKey = node.Key.Hex()
	}

	tx := snapTx.(pubTx)
	stateCIDStr, mhKey, err := tx.publishRaw(ipld.MEthStateTrie, node.Value, height)
	if err != nil {
		return err
	}

	_, err = tx.Exec(snapt.TableStateNode.ToInsertStatement(),
		height.Uint64(), headerID, stateKey, stateCIDStr, node.Path, node.NodeType, false, mhKey)
	if err != nil {
		return err
	}
	// increment state node counter.
	atomic.AddUint64(&p.stateNodeCounter, 1)
	prom.IncStateNodeCount()

	// increment current batch size counter
	p.currBatchSize += 2
	return err
}

// PublishStorageNode writes the storage node to the ipfs backing pg datastore and adds secondary indexes in the storage_cids table
func (p *publisher) PublishStorageNode(node *snapt.Node, headerID string, height *big.Int, statePath []byte, snapTx snapt.Tx) error {
	var storageKey string
	if !snapt.IsNullHash(node.Key) {
		storageKey = node.Key.Hex()
	}

	tx := snapTx.(pubTx)
	storageCIDStr, mhKey, err := tx.publishRaw(ipld.MEthStorageTrie, node.Value, height)
	if err != nil {
		return err
	}

	_, err = tx.Exec(snapt.TableStorageNode.ToInsertStatement(),
		height.Uint64(), headerID, statePath, storageKey, storageCIDStr, node.Path, node.NodeType, false, mhKey)
	if err != nil {
		return err
	}
	// increment storage node counter.
	atomic.AddUint64(&p.storageNodeCounter, 1)
	prom.IncStorageNodeCount()

	// increment current batch size counter
	p.currBatchSize += 2
	return err
}

// PublishCode writes code to the ipfs backing pg datastore
func (p *publisher) PublishCode(height *big.Int, codeHash common.Hash, codeBytes []byte, snapTx snapt.Tx) error {
	// no codec for code, doesn't matter though since blockstore key is multihash-derived
	mhKey, err := shared.MultihashKeyFromKeccak256(codeHash)
	if err != nil {
		return fmt.Errorf("error deriving multihash key from codehash: %v", err)
	}

	tx := snapTx.(pubTx)
	if _, err = tx.Exec(snapt.TableIPLDBlock.ToInsertStatement(), height.Uint64(), mhKey, codeBytes); err != nil {
		return fmt.Errorf("error publishing code IPLD: %v", err)
	}

	// increment code node counter.
	atomic.AddUint64(&p.codeNodeCounter, 1)
	prom.IncCodeNodeCount()

	p.currBatchSize++
	return nil
}

func (p *publisher) PrepareTxForBatch(tx snapt.Tx, maxBatchSize uint) (snapt.Tx, error) {
	var err error
	// maximum batch size reached, commit the current transaction and begin a new transaction.
	if maxBatchSize <= p.currBatchSize {
		if err = tx.Commit(); err != nil {
			return nil, err
		}

		snapTx, err := p.db.Begin(context.Background())
		tx = pubTx{Tx: snapTx}
		if err != nil {
			return nil, err
		}

		p.currBatchSize = 0
	}

	return tx, nil
}

// logNodeCounters periodically logs the number of node processed.
func (p *publisher) logNodeCounters() {
	t := time.NewTicker(logInterval)
	for range t.C {
		p.printNodeCounters("progress")
	}
}

func (p *publisher) printNodeCounters(msg string) {
	log.WithFields(log.Fields{
		"runtime":       time.Now().Sub(p.startTime).String(),
		"state nodes":   atomic.LoadUint64(&p.stateNodeCounter),
		"storage nodes": atomic.LoadUint64(&p.storageNodeCounter),
		"code nodes":    atomic.LoadUint64(&p.codeNodeCounter),
	}).Info(msg)
}
