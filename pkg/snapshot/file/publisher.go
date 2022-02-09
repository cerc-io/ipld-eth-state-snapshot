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
	// "bufio"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ipfs/go-cid"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	dshelp "github.com/ipfs/go-ipfs-ds-help"
	"github.com/multiformats/go-multihash"
	"github.com/sirupsen/logrus"

	"github.com/ethereum/go-ethereum/statediff/indexer/ipld"
	nodeinfo "github.com/ethereum/go-ethereum/statediff/indexer/node"
	"github.com/ethereum/go-ethereum/statediff/indexer/shared"
	snapt "github.com/vulcanize/eth-pg-ipfs-state-snapshot/pkg/types"
)

var _ snapt.Publisher = (*publisher)(nil)

var (
	// tables written once per block
	perBlockTables = []*snapt.Table{
		&snapt.TableIPLDBlock,
		&snapt.TableNodeInfo,
		&snapt.TableHeader,
	}
	// tables written during state iteration
	perNodeTables = []*snapt.Table{
		&snapt.TableIPLDBlock,
		&snapt.TableStateNode,
		&snapt.TableStorageNode,
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
type fileWriters map[*snapt.Table]fileWriter

type fileTx struct {
	fileWriters
	// index uint
}

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
	buf := file //bufio.NewWriter(file)
	ret = fileWriter{csv.NewWriter(buf)}
	return
}

func (tx fileWriters) write(tbl *snapt.Table, args ...interface{}) error {
	row := tbl.ToCsvRow(args...)
	return tx[tbl].Write(row)
}

func makeFileWriters(dir string, tables []*snapt.Table) (fileWriters, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	writers := map[*snapt.Table]fileWriter{}
	for _, tbl := range tables {
		w, err := newFileWriter(TableFile(dir, tbl.Name))
		if err != nil {
			return nil, err
		}
		writers[tbl] = w
	}
	return writers, nil
}

// NewPublisher creates a publisher which writes to per-table CSV files which can be imported
// with the Postgres COPY command.
// The output directory will be created if it does not exist.
func NewPublisher(path string, node nodeinfo.Info) (*publisher, error) {
	if err := os.MkdirAll(path, 0755); err != nil {
		return nil, err
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
	index := atomic.LoadUint32(&p.txCounter)
	dir := p.txDir(index)
	writers, err := makeFileWriters(dir, perNodeTables)
	if err != nil {
		return nil, err
	}

	atomic.AddUint32(&p.txCounter, 1)
	return fileTx{writers}, nil
}

// PublishRaw derives a cid from raw bytes and provided codec and multihash type, and writes it to the db tx
// returns the CID and blockstore prefixed multihash key
func (tx fileWriters) publishRaw(codec uint64, raw []byte) (cid, prefixedKey string, err error) {
	c, err := ipld.RawdataToCid(codec, raw, multihash.KECCAK_256)
	if err != nil {
		return
	}
	cid = c.String()
	prefixedKey, err = tx.publishIPLD(c, raw)
	return
}

func (tx fileWriters) publishIPLD(c cid.Cid, raw []byte) (string, error) {
	dbKey := dshelp.MultihashToDsKey(c.Hash())
	prefixedKey := blockstore.BlockPrefix.String() + dbKey.String()
	return prefixedKey, tx.write(&snapt.TableIPLDBlock, prefixedKey, raw)
}

// PublishHeader writes the header to the ipfs backing pg datastore and adds secondary
// indexes in the header_cids table
func (p *publisher) PublishHeader(header *types.Header) error {
	headerNode, err := ipld.NewEthHeader(header)
	if err != nil {
		return err
	}
	if _, err = p.writers.publishIPLD(headerNode.Cid(), headerNode.RawData()); err != nil {
		return err
	}

	mhKey := shared.MultihashKeyFromCID(headerNode.Cid())
	err = p.writers.write(&snapt.TableNodeInfo, p.nodeInfo.GenesisBlock, p.nodeInfo.NetworkID, p.nodeInfo.ID,
		p.nodeInfo.ClientName, p.nodeInfo.ChainID)
	if err != nil {
		return err
	}
	err = p.writers.write(&snapt.TableHeader, header.Number.String(), header.Hash().Hex(), header.ParentHash.Hex(),
		headerNode.Cid().String(), 0, p.nodeInfo.ID, 0, header.Root.Hex(), header.TxHash.Hex(),
		header.ReceiptHash.Hex(), header.UncleHash.Hex(), header.Bloom.Bytes(), header.Time, mhKey,
		0, header.Coinbase.String())
	if err != nil {
		return err
	}
	return p.writers.Commit()
}

// PublishStateNode writes the state node to the ipfs backing datastore and adds secondary indexes
// in the state_cids table
func (p *publisher) PublishStateNode(node *snapt.Node, headerID string, tx_ snapt.Tx) error {
	var stateKey string
	if !snapt.IsNullHash(node.Key) {
		stateKey = node.Key.Hex()
	}

	tx := tx_.(fileTx)
	stateCIDStr, mhKey, err := tx.publishRaw(ipld.MEthStateTrie, node.Value)
	if err != nil {
		return err
	}

	err = tx.write(&snapt.TableStateNode, headerID, stateKey, stateCIDStr, node.Path,
		node.NodeType, false, mhKey)
	if err != nil {
		return err
	}
	// increment state node counter.
	atomic.AddUint64(&p.stateNodeCounter, 1)
	// increment current batch size counter
	p.currBatchSize += 2
	return err
}

// PublishStorageNode writes the storage node to the ipfs backing pg datastore and adds secondary
// indexes in the storage_cids table
func (p *publisher) PublishStorageNode(node *snapt.Node, headerID string, statePath []byte, tx_ snapt.Tx) error {
	var storageKey string
	if !snapt.IsNullHash(node.Key) {
		storageKey = node.Key.Hex()
	}

	tx := tx_.(fileTx)
	storageCIDStr, mhKey, err := tx.publishRaw(ipld.MEthStorageTrie, node.Value)
	if err != nil {
		return err
	}

	err = tx.write(&snapt.TableStorageNode, headerID, statePath, storageKey, storageCIDStr, node.Path,
		node.NodeType, false, mhKey)
	if err != nil {
		return err
	}
	// increment storage node counter.
	atomic.AddUint64(&p.storageNodeCounter, 1)
	// increment current batch size counter
	p.currBatchSize += 2
	return nil
}

// PublishCode writes code to the ipfs backing pg datastore
func (p *publisher) PublishCode(codeHash common.Hash, codeBytes []byte, tx_ snapt.Tx) error {
	// no codec for code, doesn't matter though since blockstore key is multihash-derived
	mhKey, err := shared.MultihashKeyFromKeccak256(codeHash)
	if err != nil {
		return fmt.Errorf("error deriving multihash key from codehash: %v", err)
	}

	tx := tx_.(fileTx)
	if err = tx.write(&snapt.TableIPLDBlock, mhKey, codeBytes); err != nil {
		return fmt.Errorf("error publishing code IPLD: %v", err)
	}
	// increment code node counter.
	atomic.AddUint64(&p.codeNodeCounter, 1)
	p.currBatchSize++
	return nil
}

func (p *publisher) PrepareTxForBatch(tx snapt.Tx, maxBatchSize uint) (snapt.Tx, error) {
	return tx, nil
}

// logNodeCounters periodically logs the number of node processed.
func (p *publisher) logNodeCounters() {
	t := time.NewTicker(logInterval)
	for range t.C {
		p.printNodeCounters()
	}
}

func (p *publisher) printNodeCounters() {
	logrus.Infof("runtime: %s", time.Now().Sub(p.startTime).String())
	logrus.Infof("processed state nodes: %d", atomic.LoadUint64(&p.stateNodeCounter))
	logrus.Infof("processed storage nodes: %d", atomic.LoadUint64(&p.storageNodeCounter))
	logrus.Infof("processed code nodes: %d", atomic.LoadUint64(&p.codeNodeCounter))
}
