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

package publisher

import (
	"encoding/csv"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/lib/pq"

	"github.com/ipfs/go-cid"
	"github.com/sirupsen/logrus"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/statediff/indexer/ipld"
	"github.com/ethereum/go-ethereum/statediff/indexer/models"
	nodeinfo "github.com/ethereum/go-ethereum/statediff/indexer/node"
	"github.com/ethereum/go-ethereum/statediff/indexer/shared/schema"

	"github.com/cerc-io/ipld-eth-state-snapshot/pkg/prom"
	snapt "github.com/cerc-io/ipld-eth-state-snapshot/pkg/types"
)

var _ snapt.Publisher = (*publisher)(nil)

var (
	// tables written once per block
	perBlockTables = []*schema.Table{
		&schema.TableIPLDBlock,
		&schema.TableNodeInfo,
		&schema.TableHeader,
	}
	// tables written during state iteration
	perNodeTables = []*schema.Table{
		&schema.TableIPLDBlock,
		&schema.TableStateNode,
		&schema.TableStorageNode,
	}
)

const logInterval = 1 * time.Minute

type publisher struct {
	dir     string // dir containing output files
	writers fileWriters

	nodeInfo nodeinfo.Info

	startTime          time.Time
	currBatchSize      uint
	stateNodeCounter   uint64
	storageNodeCounter uint64
	codeNodeCounter    uint64
	txCounter          uint32
}

type fileWriter struct {
	*csv.Writer
}

// fileWriters wraps the file writers for each output table
type fileWriters map[string]fileWriter

type fileTx struct{ fileWriters }

func (tx fileWriters) Commit() error {
	for _, w := range tx {
		w.Flush()
		if err := w.Error(); err != nil {
			return err
		}
	}
	return nil
}
func (fileWriters) Rollback() error { return nil } // TODO: delete the file?

func newFileWriter(path string) (ret fileWriter, err error) {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	ret = fileWriter{csv.NewWriter(file)}
	return
}

func (tx fileWriters) write(tbl *schema.Table, args ...interface{}) error {
	row := tbl.ToCsvRow(args...)
	return tx[tbl.Name].Write(row)
}

func makeFileWriters(dir string, tables []*schema.Table) (fileWriters, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	writers := fileWriters{}
	for _, tbl := range tables {
		w, err := newFileWriter(TableFile(dir, tbl.Name))
		if err != nil {
			return nil, err
		}
		writers[tbl.Name] = w
	}
	return writers, nil
}

// NewPublisher creates a publisher which writes to per-table CSV files which can be imported
// with the Postgres COPY command.
// The output directory will be created if it does not exist.
func NewPublisher(path string, node nodeinfo.Info) (*publisher, error) {
	if err := os.MkdirAll(path, 0777); err != nil {
		return nil, fmt.Errorf("unable to make MkdirAll for path: %s err: %s", path, err)
	}
	writers, err := makeFileWriters(path, perBlockTables)
	if err != nil {
		return nil, err
	}
	pub := &publisher{
		writers:   writers,
		dir:       path,
		nodeInfo:  node,
		startTime: time.Now(),
	}
	go pub.logNodeCounters()
	return pub, nil
}

func TableFile(dir, name string) string { return filepath.Join(dir, name+".csv") }

func (p *publisher) txDir(index uint32) string {
	return filepath.Join(p.dir, fmt.Sprintf("%010d", index))
}

func (p *publisher) BeginTx() (snapt.Tx, error) {
	index := atomic.AddUint32(&p.txCounter, 1) - 1
	dir := p.txDir(index)
	writers, err := makeFileWriters(dir, perNodeTables)
	if err != nil {
		return nil, err
	}

	return fileTx{writers}, nil
}

func (tx fileWriters) publishIPLD(c cid.Cid, raw []byte, height *big.Int) error {
	return tx.write(&schema.TableIPLDBlock, height.String(), c.String(), raw)
}

// PublishIPLD writes an IPLD to the ipld.blocks blockstore
func (p *publisher) PublishIPLD(c cid.Cid, raw []byte, height *big.Int, snapTx snapt.Tx) error {
	tx := snapTx.(fileTx)
	return tx.publishIPLD(c, raw, height)
}

// PublishHeader writes the header to the ipfs backing pg datastore and adds secondary
// indexes in the header_cids table
func (p *publisher) PublishHeader(header *types.Header) error {
	headerNode, err := ipld.NewEthHeader(header)
	if err != nil {
		return err
	}
	if err := p.writers.publishIPLD(headerNode.Cid(), headerNode.RawData(), header.Number); err != nil {
		return err
	}

	err = p.writers.write(&schema.TableNodeInfo, p.nodeInfo.GenesisBlock, p.nodeInfo.NetworkID, p.nodeInfo.ID,
		p.nodeInfo.ClientName, p.nodeInfo.ChainID)
	if err != nil {
		return err
	}
	err = p.writers.write(&schema.TableHeader,
		header.Number.String(),
		header.Hash().Hex(),
		header.ParentHash.Hex(),
		headerNode.Cid().String(),
		"0",
		pq.StringArray([]string{p.nodeInfo.ID}),
		"0",
		header.Root.Hex(),
		header.TxHash.Hex(),
		header.ReceiptHash.Hex(),
		header.UncleHash.Hex(),
		header.Bloom.Bytes(),
		strconv.FormatUint(header.Time, 10),
		header.Coinbase.String(),
		true,
	)
	if err != nil {
		return err
	}
	return p.writers.Commit()
}

// PublishStateLeafNode writes the state node eth.state_cids
func (p *publisher) PublishStateLeafNode(stateNode *models.StateNodeModel, snapTx snapt.Tx) error {
	tx := snapTx.(fileTx)

	err := tx.write(&schema.TableStateNode,
		stateNode.BlockNumber,
		stateNode.HeaderID,
		stateNode.StateKey,
		stateNode.CID,
		false,
		stateNode.Balance,
		strconv.FormatUint(stateNode.Nonce, 10),
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

// PublishStorageLeafNode writes the storage node to eth.storage_cids
func (p *publisher) PublishStorageLeafNode(storageNode *models.StorageNodeModel, snapTx snapt.Tx) error {
	tx := snapTx.(fileTx)

	err := tx.write(&schema.TableStorageNode,
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
	return nil
}

func (p *publisher) PrepareTxForBatch(tx snapt.Tx, maxBatchSize uint) (snapt.Tx, error) {
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
	logrus.WithFields(logrus.Fields{
		"runtime":       time.Now().Sub(p.startTime).String(),
		"state nodes":   atomic.LoadUint64(&p.stateNodeCounter),
		"storage nodes": atomic.LoadUint64(&p.storageNodeCounter),
		"code nodes":    atomic.LoadUint64(&p.codeNodeCounter),
	}).Info(msg)
}
