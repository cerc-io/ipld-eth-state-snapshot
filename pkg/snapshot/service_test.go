package snapshot

import (
	"errors"
	"fmt"
	"math/big"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/golang/mock/gomock"

	fixt "github.com/vulcanize/ipld-eth-state-snapshot/fixture"
	mock "github.com/vulcanize/ipld-eth-state-snapshot/mocks/snapshot"
	snapt "github.com/vulcanize/ipld-eth-state-snapshot/pkg/types"
	"github.com/vulcanize/ipld-eth-state-snapshot/test"
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
		pub, tx := makeMocks(t)
		pub.EXPECT().PublishHeader(gomock.Eq(&fixt.Block1_Header))
		pub.EXPECT().BeginTx().Return(tx, nil).
			Times(workers)
		pub.EXPECT().PrepareTxForBatch(gomock.Any(), gomock.Any()).Return(tx, nil).
			AnyTimes()
		pub.EXPECT().PublishStateNode(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			// Use MinTimes as duplicate nodes are expected at boundaries
			MinTimes(len(fixt.Block1_StateNodePaths))

		// TODO: fixtures for storage node
		// pub.EXPECT().PublishStorageNode(gomock.Eq(fixt.StorageNode), gomock.Eq(int64(0)), gomock.Any())

		tx.EXPECT().Commit().
			Times(workers)

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
	}

	testCases := []int{1, 4, 8, 16, 32}
	for _, tc := range testCases {
		t.Run("case", func(t *testing.T) { runCase(t, tc) })
	}
}

type indexedNode struct {
	value     snapt.Node
	isIndexed bool
}

type storageNodeKey struct {
	statePath   string
	storagePath string
}

func TestAccountSelectiveSnapshot(t *testing.T) {
	snapShotHeight := uint64(32)
	watchedAddresses := make(map[common.Address]struct{}, 2)
	watchedAddresses[common.HexToAddress("0x825a6eec09e44Cb0fa19b84353ad0f7858d7F61a")] = struct{}{}
	watchedAddresses[common.HexToAddress("0x0616F59D291a898e796a1FAD044C5926ed2103eC")] = struct{}{}

	expectedStateNodeIndexes := []int{0, 1, 2, 6}

	statePath33 := []byte{3, 3}
	expectedStorageNodeIndexes33 := []int{0, 1, 2, 3, 4, 6, 8}

	statePath12 := []byte{12}
	expectedStorageNodeIndexes12 := []int{12, 14, 16}

	runCase := func(t *testing.T, workers int) {
		expectedStateNodes := sync.Map{}

		for _, expectedStateNodeIndex := range expectedStateNodeIndexes {
			path := fixt.Chain2_Block32_StateNodes[expectedStateNodeIndex].Path
			expectedStateNodes.Store(string(path), indexedNode{
				value:     fixt.Chain2_Block32_StateNodes[expectedStateNodeIndex],
				isIndexed: false,
			})
		}

		expectedStorageNodes := sync.Map{}

		for _, expectedStorageNodeIndex := range expectedStorageNodeIndexes33 {
			path := fixt.Chain2_Block32_StorageNodes[expectedStorageNodeIndex].Path
			key := storageNodeKey{
				statePath:   string(statePath33),
				storagePath: string(path),
			}
			value := indexedNode{
				value:     fixt.Chain2_Block32_StorageNodes[expectedStorageNodeIndex].Node,
				isIndexed: false,
			}
			expectedStorageNodes.Store(key, value)
		}

		for _, expectedStorageNodeIndex := range expectedStorageNodeIndexes12 {
			path := fixt.Chain2_Block32_StorageNodes[expectedStorageNodeIndex].Path
			key := storageNodeKey{
				statePath:   string(statePath12),
				storagePath: string(path),
			}
			value := indexedNode{
				value:     fixt.Chain2_Block32_StorageNodes[expectedStorageNodeIndex].Node,
				isIndexed: false,
			}
			expectedStorageNodes.Store(key, value)
		}

		pub, tx := makeMocks(t)
		pub.EXPECT().PublishHeader(gomock.Eq(&fixt.Chain2_Block32_Header))
		pub.EXPECT().BeginTx().Return(tx, nil).
			Times(workers)
		pub.EXPECT().PrepareTxForBatch(gomock.Any(), gomock.Any()).Return(tx, nil).
			AnyTimes()
		tx.EXPECT().Commit().
			Times(workers)
		pub.EXPECT().PublishCode(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Eq(tx)).
			AnyTimes()
		pub.EXPECT().PublishStateNode(
			gomock.Any(),
			gomock.Eq(fixt.Chain2_Block32_Header.Hash().String()),
			gomock.Eq(fixt.Chain2_Block32_Header.Number),
			gomock.Eq(tx)).
			Do(func(node *snapt.Node, _ string, _ *big.Int, _ snapt.Tx) error {
				key := string(node.Path)
				// Check published nodes
				if expectedStateNode, ok := expectedStateNodes.Load(key); ok {
					expectedVal := expectedStateNode.(indexedNode).value
					test.ExpectEqual(t, expectedVal, *node)

					// Mark expected node as indexed
					expectedStateNodes.Store(key, indexedNode{
						value:     expectedVal,
						isIndexed: true,
					})
				} else {
					t.Fatal("got unexpected node for path", node.Path)
				}
				return nil
			}).
			AnyTimes()
		pub.EXPECT().PublishStorageNode(
			gomock.Any(),
			gomock.Eq(fixt.Chain2_Block32_Header.Hash().String()),
			gomock.Eq(new(big.Int).SetUint64(snapShotHeight)),
			gomock.Any(),
			gomock.Eq(tx)).
			Do(func(node *snapt.Node, _ string, _ *big.Int, statePath []byte, _ snapt.Tx) error {
				key := storageNodeKey{
					statePath:   string(statePath),
					storagePath: string(node.Path),
				}
				// Check published nodes
				if expectedStorageNode, ok := expectedStorageNodes.Load(key); ok {
					expectedVal := expectedStorageNode.(indexedNode).value
					test.ExpectEqual(t, expectedVal, *node)

					// Mark expected node as indexed
					expectedStorageNodes.Store(key, indexedNode{
						value:     expectedVal,
						isIndexed: true,
					})
				} else {
					t.Fatal("got unexpected node for state path", statePath, "storage path", node.Path)
				}
				return nil
			}).
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
			if !value.(indexedNode).isIndexed {
				t.Fatal("state node not indexed for path", []byte(key.(string)))
				return false
			}
			return true
		})
		expectedStorageNodes.Range(func(key, value any) bool {
			if !value.(indexedNode).isIndexed {
				t.Fatal("storage node not indexed for state path", []byte(key.(storageNodeKey).statePath), "storage path", []byte(key.(storageNodeKey).storagePath))
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
		stateNodePaths := sync.Map{}
		for _, path := range fixt.Block1_StateNodePaths {
			stateNodePaths.Store(string(path), struct{}{})
		}
		var indexedStateNodesCount int32

		pub, tx := makeMocks(t)
		pub.EXPECT().PublishHeader(gomock.Eq(&fixt.Block1_Header))
		pub.EXPECT().BeginTx().Return(tx, nil).Times(workers)
		pub.EXPECT().PrepareTxForBatch(gomock.Any(), gomock.Any()).Return(tx, nil).AnyTimes()
		tx.EXPECT().Commit().Times(workers)
		pub.EXPECT().PublishStateNode(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(node *snapt.Node, _ string, _ *big.Int, _ snapt.Tx) error {
				// Start throwing an error after a certain number of state nodes have been indexed
				if indexedStateNodesCount >= interruptAt {
					return errors.New("failingPublishStateNode")
				} else {
					stateNodePaths.Delete(string(node.Path))
					atomic.AddInt32(&indexedStateNodesCount, 1)
				}
				return nil
			}).
			MaxTimes(int(interruptAt) + workers)

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
		recoveryPub.EXPECT().PublishStateNode(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(node *snapt.Node, _ string, _ *big.Int, _ snapt.Tx) error {
				stateNodePaths.Delete(string(node.Path))
				return nil
			}).
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
		stateNodePaths.Range(func(key, value any) bool {
			t.Fatal("state node not indexed for path", []byte(key.(string)))
			return false
		})
	}

	testCases := []int{1, 4, 8, 16, 32}
	numInterrupts := 3
	interrupts := make([]int32, numInterrupts)
	for i := 0; i < numInterrupts; i++ {
		rand.Seed(time.Now().UnixNano())
		interrupts[i] = 1 + rand.Int31n(int32(len(fixt.Block1_StateNodePaths)))
	}

	for _, tc := range testCases {
		for _, interrupt := range interrupts {
			t.Run(fmt.Sprint("case", tc, interrupt), func(t *testing.T) { runCase(t, tc, interrupt) })
		}
	}
}
