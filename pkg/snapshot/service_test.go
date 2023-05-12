package snapshot

import (
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/statediff/indexer/models"
	"github.com/golang/mock/gomock"

	fixt "github.com/cerc-io/ipld-eth-state-snapshot/fixture"
	mock "github.com/cerc-io/ipld-eth-state-snapshot/mocks/snapshot"
	snapt "github.com/cerc-io/ipld-eth-state-snapshot/pkg/types"
	"github.com/cerc-io/ipld-eth-state-snapshot/test"
)

var (
	stateNodeDuplicateErr    = "state node indexed multiple times (%d) for state key %v"
	storageNodeDuplicateErr  = "storage node indexed multiple times (%d) for state key %v and storage key %v"
	stateNodeNotIndexedErr   = "state node not indexed for state key %v"
	storageNodeNotIndexedErr = "storage node not indexed for state key %v, storage key %v"

	unexpectedStateNodeErr   = "got unexpected state node for state key %v"
	unexpectedStorageNodeErr = "got unexpected storage node for state key %v, storage key %v"

	extraNodesIndexedErr = "number of nodes indexed (%v) is more than expected (max %v)"
)

func testConfig(leveldbpath, ancientdbpath string) *Config {
	return &Config{
		Eth: &EthConfig{
			LevelDBPath:   leveldbpath,
			AncientDBPath: ancientdbpath,
			NodeInfo:      test.DefaultNodeInfo,
		},
		DB: &DBConfig{
			URI:        test.DefaultPgConfig.DbConnectionString(),
			ConnConfig: test.DefaultPgConfig,
		},
	}
}

func makeMocks(t *testing.T) (*mock.MockPublisher, *mock.MockTx) {
	ctl := gomock.NewController(t)
	pub := mock.NewMockPublisher(ctl)
	tx := mock.NewMockTx(ctl)
	return pub, tx
}

func TestCreateSnapshot(t *testing.T) {
	runCase := func(t *testing.T, workers int) {
		expectedStateLeafKeys := sync.Map{}
		for _, key := range fixt.Block1_StateNodeLeafKeys {
			expectedStateLeafKeys.Store(key, struct{}{})
		}

		pub, tx := makeMocks(t)
		pub.EXPECT().PublishHeader(gomock.Eq(&fixt.Block1_Header))
		pub.EXPECT().BeginTx().Return(tx, nil).
			Times(workers)
		pub.EXPECT().PrepareTxForBatch(gomock.Any(), gomock.Any()).Return(tx, nil).
			AnyTimes()
		tx.EXPECT().Commit().
			Times(workers)
		pub.EXPECT().PublishStateLeafNode(
			gomock.Any(),
			gomock.Eq(tx)).
			Do(func(stateNode *models.StateNodeModel, _ snapt.Tx) error {
				if stateNode.BlockNumber != fixt.Block1_Header.Number.String() {
					t.Fatalf(unexpectedStateNodeErr, stateNode.StateKey)
				}
				if stateNode.HeaderID != fixt.Block1_Header.Hash().String() {
					t.Fatalf(unexpectedStateNodeErr, stateNode.StateKey)
				}
				if _, ok := expectedStateLeafKeys.Load(stateNode.StateKey); ok {
					expectedStateLeafKeys.Delete(stateNode.StateKey)
				} else {
					t.Fatalf(unexpectedStateNodeErr, stateNode.StateKey)
				}
				return nil
			}).
			AnyTimes()
		pub.EXPECT().PublishIPLD(gomock.Any(), gomock.Any(), gomock.Eq(fixt.Block1_Header.Number), gomock.Eq(tx)).
			AnyTimes()
		// Note: block 1 doesn't have storage nodes. TODO: add fixtures with storage nodes

		chainDataPath, ancientDataPath := fixt.GetChainDataPath("chaindata")
		config := testConfig(chainDataPath, ancientDataPath)
		edb, err := NewLevelDB(config.Eth)
		if err != nil {
			t.Fatal(err)
		}
		defer edb.Close()

		recovery := filepath.Join(t.TempDir(), "recover.csv")
		service, err := NewSnapshotService(edb, pub, recovery)
		if err != nil {
			t.Fatal(err)
		}

		params := SnapshotParams{Height: 1, Workers: uint(workers)}
		err = service.CreateSnapshot(params)
		if err != nil {
			t.Fatal(err)
		}

		// Check if all expected state nodes are indexed
		expectedStateLeafKeys.Range(func(key, value any) bool {
			t.Fatalf(stateNodeNotIndexedErr, key.(string))
			return true
		})
	}

	testCases := []int{1, 4, 8, 16, 32}
	for _, tc := range testCases {
		t.Run("case", func(t *testing.T) { runCase(t, tc) })
	}
}

type indexedStateLeafNode struct {
	value     models.StateNodeModel
	isIndexed bool
}

type indexedStorageLeafNode struct {
	value     models.StorageNodeModel
	isIndexed bool
}

type storageNodeKey struct {
	stateKey   string
	storageKey string
}

func TestAccountSelectiveSnapshot(t *testing.T) {
	snapShotHeight := uint64(32)
	watchedAddresses := map[common.Address]struct{}{
		common.HexToAddress("0x825a6eec09e44Cb0fa19b84353ad0f7858d7F61a"): {},
		common.HexToAddress("0x0616F59D291a898e796a1FAD044C5926ed2103eC"): {},
	}
	expectedStateNodeIndexes := []int{0, 4}

	stateKey33 := common.HexToHash("0x33153abc667e873b6036c8a46bdd847e2ade3f89b9331c78ef2553fea194c50d").String()
	expectedStorageNodeIndexes33 := []int{0, 1, 2, 4, 6}

	stateKey12 := common.HexToHash("0xcabc5edb305583e33f66322ceee43088aa99277da772feb5053512d03a0a702b").String()
	expectedStorageNodeIndexes12 := []int{9, 11}

	runCase := func(t *testing.T, workers int) {
		expectedStateNodes := sync.Map{}

		for _, expectedStateNodeIndex := range expectedStateNodeIndexes {
			key := fixt.Chain2_Block32_StateNodes[expectedStateNodeIndex].StateKey
			expectedStateNodes.Store(key, indexedStateLeafNode{
				value:     fixt.Chain2_Block32_StateNodes[expectedStateNodeIndex],
				isIndexed: false,
			})
		}

		expectedStorageNodes := sync.Map{}

		for _, expectedStorageNodeIndex := range expectedStorageNodeIndexes33 {
			key := fixt.Chain2_Block32_StorageNodes[expectedStorageNodeIndex].StorageKey
			keys := storageNodeKey{
				stateKey:   stateKey33,
				storageKey: key,
			}
			value := indexedStorageLeafNode{
				value:     fixt.Chain2_Block32_StorageNodes[expectedStorageNodeIndex],
				isIndexed: false,
			}
			expectedStorageNodes.Store(keys, value)
		}

		for _, expectedStorageNodeIndex := range expectedStorageNodeIndexes12 {
			key := fixt.Chain2_Block32_StorageNodes[expectedStorageNodeIndex].StorageKey
			keys := storageNodeKey{
				stateKey:   stateKey12,
				storageKey: key,
			}
			value := indexedStorageLeafNode{
				value:     fixt.Chain2_Block32_StorageNodes[expectedStorageNodeIndex],
				isIndexed: false,
			}
			expectedStorageNodes.Store(keys, value)
		}

		var count int

		pub, tx := makeMocks(t)
		pub.EXPECT().PublishHeader(gomock.Eq(&fixt.Chain2_Block32_Header))
		pub.EXPECT().BeginTx().Return(tx, nil).
			Times(workers)
		pub.EXPECT().PrepareTxForBatch(gomock.Any(), gomock.Any()).Return(tx, nil).
			AnyTimes()
		tx.EXPECT().Commit().
			Times(workers)
		pub.EXPECT().PublishStateLeafNode(
			gomock.Any(),
			gomock.Eq(tx)).
			Do(func(stateNode *models.StateNodeModel, _ snapt.Tx) error {
				count++
				if stateNode.BlockNumber != fixt.Chain2_Block32_Header.Number.String() {
					t.Fatalf(unexpectedStateNodeErr, stateNode.StateKey)
				}
				if stateNode.HeaderID != fixt.Chain2_Block32_Header.Hash().String() {
					t.Fatalf(unexpectedStateNodeErr, stateNode.StateKey)
				}
				key := stateNode.StateKey
				// Check published nodes
				if expectedStateNode, ok := expectedStateNodes.Load(key); ok {
					expectedVal := expectedStateNode.(indexedStateLeafNode).value
					test.ExpectEqual(t, expectedVal, *stateNode)

					// Mark expected node as indexed
					expectedStateNodes.Store(key, indexedStateLeafNode{
						value:     expectedVal,
						isIndexed: true,
					})
				} else {
					t.Fatalf(unexpectedStateNodeErr, stateNode.StateKey)
				}
				return nil
			}).
			AnyTimes()
		pub.EXPECT().PublishStorageLeafNode(
			gomock.Any(),
			gomock.Eq(tx)).
			Do(func(storageNode *models.StorageNodeModel, _ snapt.Tx) error {
				if storageNode.BlockNumber != fixt.Chain2_Block32_Header.Number.String() {
					t.Fatalf(unexpectedStorageNodeErr, storageNode.StateKey, storageNode.StorageKey)
				}
				if storageNode.HeaderID != fixt.Chain2_Block32_Header.Hash().String() {
					t.Fatalf(unexpectedStorageNodeErr, storageNode.StateKey, storageNode.StorageKey)
				}
				key := storageNodeKey{
					stateKey:   storageNode.StateKey,
					storageKey: storageNode.StorageKey,
				}
				// Check published nodes
				if expectedStorageNode, ok := expectedStorageNodes.Load(key); ok {
					expectedVal := expectedStorageNode.(indexedStorageLeafNode).value
					test.ExpectEqual(t, expectedVal, *storageNode)

					// Mark expected node as indexed
					expectedStorageNodes.Store(key, indexedStorageLeafNode{
						value:     expectedVal,
						isIndexed: true,
					})
				} else {
					t.Fatalf(unexpectedStorageNodeErr, storageNode.StateKey, storageNode.StorageKey)
				}
				return nil
			}).
			AnyTimes()
		pub.EXPECT().PublishIPLD(gomock.Any(), gomock.Any(), gomock.Eq(fixt.Chain2_Block32_Header.Number), gomock.Eq(tx)).
			AnyTimes()

		chainDataPath, ancientDataPath := fixt.GetChainDataPath("chain2data")
		config := testConfig(chainDataPath, ancientDataPath)
		edb, err := NewLevelDB(config.Eth)
		if err != nil {
			t.Fatal(err)
		}
		defer edb.Close()

		recovery := filepath.Join(t.TempDir(), "recover.csv")
		service, err := NewSnapshotService(edb, pub, recovery)
		if err != nil {
			t.Fatal(err)
		}

		params := SnapshotParams{Height: snapShotHeight, Workers: uint(workers), WatchedAddresses: watchedAddresses}
		err = service.CreateSnapshot(params)
		if err != nil {
			t.Fatal(err)
		}

		expectedStateNodes.Range(func(key, value any) bool {
			if !value.(indexedStateLeafNode).isIndexed {
				t.Fatalf(stateNodeNotIndexedErr, key)
				return false
			}
			return true
		})
		expectedStorageNodes.Range(func(key, value any) bool {
			if !value.(indexedStorageLeafNode).isIndexed {
				t.Fatalf(storageNodeNotIndexedErr, key.(storageNodeKey).stateKey, key.(storageNodeKey).storageKey)
				return false
			}
			return true
		})
	}

	testCases := []int{1, 4, 8, 16, 32}
	for _, tc := range testCases {
		t.Run("case", func(t *testing.T) { runCase(t, tc) })
	}
}

func TestRecovery(t *testing.T) {
	runCase := func(t *testing.T, workers int, interruptAt int32) {
		// map: expected state path -> number of times it got published
		expectedStateNodeKeys := sync.Map{}
		for _, key := range fixt.Block1_StateNodeLeafKeys {
			expectedStateNodeKeys.Store(key, 0)
		}
		var indexedStateNodesCount int32

		pub, tx := makeMocks(t)
		pub.EXPECT().PublishHeader(gomock.Eq(&fixt.Block1_Header))
		pub.EXPECT().BeginTx().Return(tx, nil).MaxTimes(workers)
		pub.EXPECT().PrepareTxForBatch(gomock.Any(), gomock.Any()).Return(tx, nil).AnyTimes()
		tx.EXPECT().Commit().MaxTimes(workers)
		pub.EXPECT().PublishStateLeafNode(
			gomock.Any(),
			gomock.Eq(tx)).
			DoAndReturn(func(stateNode *models.StateNodeModel, _ snapt.Tx) error {
				if stateNode.BlockNumber != fixt.Block1_Header.Number.String() {
					t.Fatalf(unexpectedStateNodeErr, stateNode.StateKey)
				}
				if stateNode.HeaderID != fixt.Block1_Header.Hash().String() {
					t.Fatalf(unexpectedStateNodeErr, stateNode.StateKey)
				}
				// Start throwing an error after a certain number of state nodes have been indexed
				if indexedStateNodesCount >= interruptAt {
					return errors.New("failingPublishStateLeafNode")
				} else {
					if prevCount, ok := expectedStateNodeKeys.Load(stateNode.StateKey); ok {
						expectedStateNodeKeys.Store(stateNode.StateKey, prevCount.(int)+1)
						atomic.AddInt32(&indexedStateNodesCount, 1)
					} else {
						t.Fatalf(unexpectedStateNodeErr, stateNode.StateKey)
					}
				}
				return nil
			}).
			MaxTimes(int(interruptAt) + workers)
		pub.EXPECT().PublishIPLD(gomock.Any(), gomock.Any(), gomock.Eq(fixt.Block1_Header.Number), gomock.Eq(tx)).
			AnyTimes()

		chainDataPath, ancientDataPath := fixt.GetChainDataPath("chaindata")
		config := testConfig(chainDataPath, ancientDataPath)
		edb, err := NewLevelDB(config.Eth)
		if err != nil {
			t.Fatal(err)
		}
		defer edb.Close()

		recovery := filepath.Join(t.TempDir(), "recover.csv")
		service, err := NewSnapshotService(edb, pub, recovery)
		if err != nil {
			t.Fatal(err)
		}

		params := SnapshotParams{Height: 1, Workers: uint(workers)}
		err = service.CreateSnapshot(params)
		if err == nil {
			t.Fatal("expected an error")
		}

		if _, err = os.Stat(recovery); err != nil {
			t.Fatal("cannot stat recovery file:", err)
		}

		// Create new mocks for recovery
		recoveryPub, tx := makeMocks(t)
		recoveryPub.EXPECT().PublishHeader(gomock.Eq(&fixt.Block1_Header))
		recoveryPub.EXPECT().BeginTx().Return(tx, nil).AnyTimes()
		recoveryPub.EXPECT().PrepareTxForBatch(gomock.Any(), gomock.Any()).Return(tx, nil).AnyTimes()
		tx.EXPECT().Commit().AnyTimes()
		recoveryPub.EXPECT().PublishStateLeafNode(
			gomock.Any(),
			gomock.Eq(tx)).
			DoAndReturn(func(stateNode *models.StateNodeModel, _ snapt.Tx) error {
				if stateNode.BlockNumber != fixt.Block1_Header.Number.String() {
					t.Fatalf(unexpectedStateNodeErr, stateNode.StateKey)
				}
				if stateNode.HeaderID != fixt.Block1_Header.Hash().String() {
					t.Fatalf(unexpectedStateNodeErr, stateNode.StateKey)
				}
				if prevCount, ok := expectedStateNodeKeys.Load(stateNode.StateKey); ok {
					expectedStateNodeKeys.Store(stateNode.StateKey, prevCount.(int)+1)
					atomic.AddInt32(&indexedStateNodesCount, 1)
				} else {
					t.Fatalf(unexpectedStateNodeErr, stateNode.StateKey)
				}
				return nil
			}).
			AnyTimes()
		recoveryPub.EXPECT().PublishIPLD(gomock.Any(), gomock.Any(), gomock.Eq(fixt.Block1_Header.Number), gomock.Eq(tx)).
			AnyTimes()

		// Create a new snapshot service for recovery
		recoveryService, err := NewSnapshotService(edb, recoveryPub, recovery)
		if err != nil {
			t.Fatal(err)
		}
		err = recoveryService.CreateSnapshot(params)
		if err != nil {
			t.Fatal(err)
		}

		// Check if recovery file has been deleted
		_, err = os.Stat(recovery)
		if err == nil {
			t.Fatal("recovery file still present")
		} else {
			if !os.IsNotExist(err) {
				t.Fatal(err)
			}
		}

		// Check if all state nodes are indexed after recovery
		expectedStateNodeKeys.Range(func(key, value any) bool {
			if value.(int) == 0 {
				t.Fatalf(stateNodeNotIndexedErr, key.(string))
			}
			return true
		})

		// nodes along the recovery path get reindexed
		maxStateNodesCount := len(fixt.Block1_StateNodeLeafKeys)
		if indexedStateNodesCount > int32(maxStateNodesCount) {
			t.Fatalf(extraNodesIndexedErr, indexedStateNodesCount, maxStateNodesCount)
		}
	}

	testCases := []int{1, 2, 4, 8, 16, 32}
	numInterrupts := 3
	interrupts := make([]int32, numInterrupts)
	for i := 0; i < numInterrupts; i++ {
		rand.Seed(time.Now().UnixNano())
		interrupts[i] = rand.Int31n(int32(len(fixt.Block1_StateNodeLeafKeys) / 2))
	}

	for _, tc := range testCases {
		for _, interrupt := range interrupts {
			t.Run(fmt.Sprint("case", tc, interrupt), func(t *testing.T) { runCase(t, tc, interrupt) })
		}
	}
}

func TestAccountSelectiveRecovery(t *testing.T) {
	snapShotHeight := uint64(32)
	watchedAddresses := map[common.Address]struct{}{
		common.HexToAddress("0x825a6eec09e44Cb0fa19b84353ad0f7858d7F61a"): {},
		common.HexToAddress("0x0616F59D291a898e796a1FAD044C5926ed2103eC"): {},
	}

	expectedStateNodeIndexes := []int{0, 4}
	expectedStorageNodeIndexes := []int{0, 1, 2, 4, 6, 9, 11}

	runCase := func(t *testing.T, workers int, interruptAt int32) {
		// map: expected state path -> number of times it got published
		expectedStateNodeKeys := sync.Map{}
		for _, expectedStateNodeIndex := range expectedStateNodeIndexes {
			key := fixt.Chain2_Block32_StateNodes[expectedStateNodeIndex].StateKey
			expectedStateNodeKeys.Store(key, 0)
		}
		expectedStorageNodeKeys := sync.Map{}
		for _, expectedStorageNodeIndex := range expectedStorageNodeIndexes {
			stateKey := fixt.Chain2_Block32_StorageNodes[expectedStorageNodeIndex].StateKey
			storageKey := fixt.Chain2_Block32_StorageNodes[expectedStorageNodeIndex].StorageKey
			keys := storageNodeKey{
				stateKey:   stateKey,
				storageKey: storageKey,
			}
			expectedStorageNodeKeys.Store(keys, 0)
		}
		var indexedStateNodesCount, indexedStorageNodesCount int32

		pub, tx := makeMocks(t)
		pub.EXPECT().PublishHeader(gomock.Eq(&fixt.Chain2_Block32_Header))
		pub.EXPECT().BeginTx().Return(tx, nil).Times(workers)
		pub.EXPECT().PrepareTxForBatch(gomock.Any(), gomock.Any()).Return(tx, nil).AnyTimes()
		tx.EXPECT().Commit().Times(workers)
		pub.EXPECT().PublishStateLeafNode(
			gomock.Any(),
			gomock.Eq(tx)).
			DoAndReturn(func(stateNode *models.StateNodeModel, _ snapt.Tx) error {
				if stateNode.BlockNumber != fixt.Chain2_Block32_Header.Number.String() {
					t.Fatalf(unexpectedStateNodeErr, stateNode.StateKey)
				}
				if stateNode.HeaderID != fixt.Chain2_Block32_Header.Hash().String() {
					t.Fatalf(unexpectedStateNodeErr, stateNode.StateKey)
				}
				// Start throwing an error after a certain number of state nodes have been indexed
				if indexedStateNodesCount >= interruptAt {
					return errors.New("failingPublishStateLeafNode")
				} else {
					if prevCount, ok := expectedStateNodeKeys.Load(stateNode.StateKey); ok {
						expectedStateNodeKeys.Store(stateNode.StateKey, prevCount.(int)+1)
						atomic.AddInt32(&indexedStateNodesCount, 1)
					} else {
						t.Fatalf(unexpectedStateNodeErr, stateNode.StateKey)
					}
				}
				return nil
			}).
			MaxTimes(int(interruptAt) + workers)
		pub.EXPECT().PublishStorageLeafNode(
			gomock.Any(),
			gomock.Eq(tx)).
			Do(func(storageNode *models.StorageNodeModel, _ snapt.Tx) error {
				if storageNode.BlockNumber != fixt.Chain2_Block32_Header.Number.String() {
					t.Fatalf(unexpectedStorageNodeErr, storageNode.StateKey, storageNode.StorageKey)
				}
				if storageNode.HeaderID != fixt.Chain2_Block32_Header.Hash().String() {
					t.Fatalf(unexpectedStorageNodeErr, storageNode.StateKey, storageNode.StorageKey)
				}
				keys := storageNodeKey{
					stateKey:   storageNode.StateKey,
					storageKey: storageNode.StorageKey,
				}
				if prevCount, ok := expectedStorageNodeKeys.Load(keys); ok {
					expectedStorageNodeKeys.Store(keys, prevCount.(int)+1)
					atomic.AddInt32(&indexedStorageNodesCount, 1)
				} else {
					t.Fatalf(unexpectedStorageNodeErr, storageNode.StateKey, storageNode.StorageKey)
				}
				return nil
			}).
			AnyTimes()
		pub.EXPECT().PublishIPLD(gomock.Any(), gomock.Any(), gomock.Eq(fixt.Chain2_Block32_Header.Number), gomock.Eq(tx)).
			AnyTimes()

		chainDataPath, ancientDataPath := fixt.GetChainDataPath("chain2data")
		config := testConfig(chainDataPath, ancientDataPath)
		edb, err := NewLevelDB(config.Eth)
		if err != nil {
			t.Fatal(err)
		}
		defer edb.Close()

		recovery := filepath.Join(t.TempDir(), "recover.csv")
		service, err := NewSnapshotService(edb, pub, recovery)
		if err != nil {
			t.Fatal(err)
		}

		params := SnapshotParams{Height: snapShotHeight, Workers: uint(workers), WatchedAddresses: watchedAddresses}
		err = service.CreateSnapshot(params)
		if err == nil {
			t.Fatal("expected an error")
		}

		if _, err = os.Stat(recovery); err != nil {
			t.Fatal("cannot stat recovery file:", err)
		}

		// Create new mocks for recovery
		recoveryPub, tx := makeMocks(t)
		recoveryPub.EXPECT().PublishHeader(gomock.Eq(&fixt.Chain2_Block32_Header))
		recoveryPub.EXPECT().BeginTx().Return(tx, nil).MaxTimes(workers)
		recoveryPub.EXPECT().PrepareTxForBatch(gomock.Any(), gomock.Any()).Return(tx, nil).AnyTimes()
		tx.EXPECT().Commit().MaxTimes(workers)
		recoveryPub.EXPECT().PublishStateLeafNode(
			gomock.Any(),
			gomock.Eq(tx)).
			DoAndReturn(func(stateNode *models.StateNodeModel, _ snapt.Tx) error {
				if stateNode.BlockNumber != fixt.Chain2_Block32_Header.Number.String() {
					t.Fatalf(unexpectedStateNodeErr, stateNode.StateKey)
				}
				if stateNode.HeaderID != fixt.Chain2_Block32_Header.Hash().String() {
					t.Fatalf(unexpectedStateNodeErr, stateNode.StateKey)
				}
				if prevCount, ok := expectedStateNodeKeys.Load(stateNode.StateKey); ok {
					expectedStateNodeKeys.Store(stateNode.StateKey, prevCount.(int)+1)
					atomic.AddInt32(&indexedStateNodesCount, 1)
				} else {
					t.Fatalf(unexpectedStateNodeErr, stateNode.StateKey)
				}
				return nil
			}).
			AnyTimes()
		recoveryPub.EXPECT().PublishStorageLeafNode(
			gomock.Any(),
			gomock.Eq(tx)).
			Do(func(storageNode *models.StorageNodeModel, _ snapt.Tx) error {
				if storageNode.BlockNumber != fixt.Chain2_Block32_Header.Number.String() {
					t.Fatalf(unexpectedStorageNodeErr, storageNode.StateKey, storageNode.StorageKey)
				}
				if storageNode.HeaderID != fixt.Chain2_Block32_Header.Hash().String() {
					t.Fatalf(unexpectedStorageNodeErr, storageNode.StateKey, storageNode.StorageKey)
				}
				keys := storageNodeKey{
					stateKey:   storageNode.StateKey,
					storageKey: storageNode.StorageKey,
				}
				if prevCount, ok := expectedStorageNodeKeys.Load(keys); ok {
					expectedStorageNodeKeys.Store(keys, prevCount.(int)+1)
					atomic.AddInt32(&indexedStorageNodesCount, 1)
				} else {
					t.Fatalf(unexpectedStorageNodeErr, storageNode.StateKey, storageNode.StorageKey)
				}
				return nil
			}).
			AnyTimes()
		recoveryPub.EXPECT().PublishIPLD(gomock.Any(), gomock.Any(), gomock.Eq(fixt.Chain2_Block32_Header.Number), gomock.Eq(tx)).
			AnyTimes()

		// Create a new snapshot service for recovery
		recoveryService, err := NewSnapshotService(edb, recoveryPub, recovery)
		if err != nil {
			t.Fatal(err)
		}
		err = recoveryService.CreateSnapshot(params)
		if err != nil {
			t.Fatal(err)
		}

		// Check if recovery file has been deleted
		_, err = os.Stat(recovery)
		if err == nil {
			t.Fatal("recovery file still present")
		} else {
			if !os.IsNotExist(err) {
				t.Fatal(err)
			}
		}

		// Check if all expected state nodes are indexed after recovery, but not in duplicate
		expectedStateNodeKeys.Range(func(key, value any) bool {
			if value.(int) == 0 {
				t.Fatalf(stateNodeNotIndexedErr, key.(string))
			}
			/* TODO: fix/figure out
			if value.(int) > 1 {
				t.Fatalf(stateNodeDuplicateErr, value.(int), key.(string))
			}
			*/
			return true
		})
		expectedStorageNodeKeys.Range(func(key, value any) bool {
			if value.(int) == 0 {
				t.Fatalf(storageNodeNotIndexedErr, key.(storageNodeKey).stateKey, key.(storageNodeKey).storageKey)
			}
			/* TODO: fix/figure out
			if value.(int) > 1 {
				t.Fatalf(storageNodeDuplicateErr, value.(int), key.(storageNodeKey).stateKey, key.(storageNodeKey).storageKey)
			}
			*/
			return true
		})

		maxStateNodesCount := len(expectedStateNodeIndexes) + workers
		if indexedStateNodesCount > int32(maxStateNodesCount) {
			t.Fatalf(extraNodesIndexedErr, indexedStateNodesCount, maxStateNodesCount)
		}
	}

	testCases := []int{1, 2, 4, 8, 16, 32}

	for _, tc := range testCases {
		t.Run(fmt.Sprint("case", tc, 1), func(t *testing.T) { runCase(t, tc, 1) })
	}
}
