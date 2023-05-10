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
	"math/big"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/lib/pq"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/statediff/indexer/database/sql"
	"github.com/ethereum/go-ethereum/statediff/indexer/database/sql/postgres"
	"github.com/ethereum/go-ethereum/statediff/indexer/ipld"
	"github.com/ethereum/go-ethereum/statediff/indexer/models"
	"github.com/ethereum/go-ethereum/statediff/indexer/shared/schema"

	"github.com/cerc-io/ipld-eth-state-snapshot/pkg/prom"
	snapt "github.com/cerc-io/ipld-eth-state-snapshot/pkg/types"
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

func (tx pubTx) publishIPLD(c cid.Cid, raw []byte, height *big.Int) error {
	_, err := tx.Exec(schema.TableIPLDBlock.ToInsertStatement(false), height.Uint64(), c.String(), raw)
	return err
}

// PublishIPLD writes an IPLD to the ipld.blocks blockstore
func (p *publisher) PublishIPLD(c cid.Cid, raw []byte, height *big.Int, snapTx snapt.Tx) error {
	tx := snapTx.(pubTx)
	return tx.publishIPLD(c, raw, height)
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

	if err := tx.publishIPLD(headerNode.Cid(), headerNode.RawData(), header.Number); err != nil {
		return err
	}

	_, err = tx.Exec(schema.TableHeader.ToInsertStatement(false),
		header.Number.Uint64(),
		header.Hash().Hex(),
		header.ParentHash.Hex(),
		headerNode.Cid().String(),
		"0",
		pq.StringArray([]string{p.db.NodeID()}),
		"0",
		header.Root.Hex(),
		header.TxHash.Hex(),
		header.ReceiptHash.Hex(),
		header.UncleHash.Hex(),
		header.Bloom.Bytes(),
		strconv.FormatUint(header.Time, 10),
		header.Coinbase.String())
	return err
}

// PublishStateLeafNode writes the state leaf node to eth.state_cids
func (p *publisher) PublishStateLeafNode(stateNode *models.StateNodeModel, snapTx snapt.Tx) error {
	tx := snapTx.(pubTx)
	_, err := tx.Exec(schema.TableStateNode.ToInsertStatement(false),
		stateNode.BlockNumber,
		stateNode.HeaderID,
		stateNode.StateKey,
		stateNode.CID,
		false,
		stateNode.Balance,
		stateNode.Nonce,
		stateNode.CodeHash,
		stateNode.StorageRoot,
		false)
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

// PublishStorageLeafNode writes the storage leaf node to eth.storage_cids
func (p *publisher) PublishStorageLeafNode(storageNode *models.StorageNodeModel, snapTx snapt.Tx) error {
	tx := snapTx.(pubTx)
	_, err := tx.Exec(schema.TableStorageNode.ToInsertStatement(false),
		storageNode.BlockNumber,
		storageNode.HeaderID,
		storageNode.StateKey,
		storageNode.StorageKey,
		storageNode.CID,
		false,
		storageNode.Value,
		false)
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
