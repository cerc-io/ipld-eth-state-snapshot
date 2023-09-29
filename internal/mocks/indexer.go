package mocks

import (
	"context"
	"fmt"
	"math/big"
	"sync"
	"testing"

	"github.com/cerc-io/plugeth-statediff/indexer"
	sdtypes "github.com/cerc-io/plugeth-statediff/types"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/golang/mock/gomock"
)

// Indexer just caches data but wraps a gomock instance, so we can mock other methods if needed
type Indexer struct {
	*MockgenIndexer
	sync.RWMutex

	IndexerData
}

type IndexerData struct {
	Headers    map[uint64]*types.Header
	StateNodes []sdtypes.StateLeafNode
	IPLDs      []sdtypes.IPLD
}

// no-op mock Batch
type Batch struct{}

// NewIndexer returns a mock indexer that caches data in lists
func NewIndexer(t *testing.T) *Indexer {
	ctl := gomock.NewController(t)
	return &Indexer{
		MockgenIndexer: NewMockgenIndexer(ctl),
		IndexerData: IndexerData{
			Headers: make(map[uint64]*types.Header),
		},
	}
}

func (i *Indexer) PushHeader(_ indexer.Batch, header *types.Header, _, _ *big.Int) (string, error) {
	i.Lock()
	defer i.Unlock()
	i.Headers[header.Number.Uint64()] = header
	return header.Hash().String(), nil
}

func (i *Indexer) PushStateNode(_ indexer.Batch, stateNode sdtypes.StateLeafNode, _ string) error {
	i.Lock()
	defer i.Unlock()
	i.StateNodes = append(i.StateNodes, stateNode)
	return nil
}

func (i *Indexer) PushIPLD(_ indexer.Batch, ipld sdtypes.IPLD) error {
	i.Lock()
	defer i.Unlock()
	i.IPLDs = append(i.IPLDs, ipld)
	return nil
}

func (i *Indexer) BeginTx(_ *big.Int, _ context.Context) indexer.Batch {
	return Batch{}
}

func (Batch) Submit() error           { return nil }
func (Batch) BlockNumber() string     { return "0" }
func (Batch) RollbackOnFailure(error) {}

// InterruptingIndexer triggers an artificial failure at a specific node count
type InterruptingIndexer struct {
	*Indexer

	InterruptAfter uint
}

func (i *InterruptingIndexer) PushStateNode(b indexer.Batch, stateNode sdtypes.StateLeafNode, h string) error {
	i.RLock()
	indexedCount := len(i.StateNodes)
	i.RUnlock()
	if indexedCount >= int(i.InterruptAfter) {
		return fmt.Errorf("mock interrupt")
	}
	return i.Indexer.PushStateNode(b, stateNode, h)
}
