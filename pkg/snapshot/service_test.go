package snapshot

import (
	"errors"
	"math/big"
	"os"
	"path/filepath"
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

	testCases := []int{1, 4, 16, 32}
	for _, tc := range testCases {
		t.Run("case", func(t *testing.T) { runCase(t, tc) })
	}
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
		expectedStateNodePaths := make(map[string]bool, 4)
		expectedStateNodes := make(map[string]snapt.Node, 4)
		for _, expectedStateNodeIndex := range expectedStateNodeIndexes {
			path := fixt.Chain2_Block32_StateNodes[expectedStateNodeIndex].Path
			expectedStateNodePaths[string(path)] = false
			expectedStateNodes[string(path)] = fixt.Chain2_Block32_StateNodes[expectedStateNodeIndex]
		}

		expectedStorageNodePaths := make(map[string]map[string]bool, 2)
		expectedStorageNodes := make(map[string]map[string]snapt.Node, 2)

		expectedStorageNodePaths[string(statePath33)] = make(map[string]bool, 7)
		expectedStorageNodes[string(statePath33)] = make(map[string]snapt.Node, 7)
		for _, expectedStorageNodeIndex := range expectedStorageNodeIndexes33 {
			path := fixt.Chain2_Block32_StorageNodes[expectedStorageNodeIndex].Path
			expectedStorageNodePaths[string(statePath33)][string(path)] = false
			expectedStorageNodes[string(statePath33)][string(path)] = fixt.Chain2_Block32_StorageNodes[expectedStorageNodeIndex].Node
		}

		expectedStorageNodePaths[string(statePath12)] = make(map[string]bool, 3)
		expectedStorageNodes[string(statePath12)] = make(map[string]snapt.Node, 3)
		for _, expectedStorageNodeIndex := range expectedStorageNodeIndexes12 {
			path := fixt.Chain2_Block32_StorageNodes[expectedStorageNodeIndex].Path
			expectedStorageNodePaths[string(statePath12)][string(path)] = false
			expectedStorageNodes[string(statePath12)][string(path)] = fixt.Chain2_Block32_StorageNodes[expectedStorageNodeIndex].Node
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
				// Check published nodes
				if expectedVal, ok := expectedStateNodes[string(node.Path)]; ok {
					test.ExpectEqual(t, expectedVal, *node)
					// Mark expected node as found
					expectedStateNodePaths[string(node.Path)] = true
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
				// Check published nodes
				if expectedVal, ok := expectedStorageNodes[string(statePath)][string(node.Path)]; ok {
					test.ExpectEqual(t, expectedVal, *node)
					// Mark expected node as found
					expectedStorageNodePaths[string(statePath)][string(node.Path)] = true
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

		for path, found := range expectedStateNodePaths {
			if !found {
				t.Fatal("state node not indexed for path", []byte(path))
			}
		}
		for statePath, expectedStateStorageNodePaths := range expectedStorageNodePaths {
			for path, found := range expectedStateStorageNodePaths {
				if !found {
					t.Fatal("storage node not indexed for state path", statePath, "storage path", []byte(path))
				}
			}
		}
	}

	testCases := []int{1, 4, 8, 16, 32}
	for _, tc := range testCases {
		t.Run("case", func(t *testing.T) { runCase(t, tc) })
	}
}

func failingPublishStateNode(_ *snapt.Node, _ string, _ *big.Int, _ snapt.Tx) error {
	return errors.New("failingPublishStateNode")
}

func TestRecovery(t *testing.T) {
	runCase := func(t *testing.T, workers int) {
		pub, tx := makeMocks(t)
		pub.EXPECT().PublishHeader(gomock.Any()).AnyTimes()
		pub.EXPECT().BeginTx().Return(tx, nil).AnyTimes()
		pub.EXPECT().PrepareTxForBatch(gomock.Any(), gomock.Any()).Return(tx, nil).AnyTimes()
		pub.EXPECT().PublishStateNode(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Times(workers).
			DoAndReturn(failingPublishStateNode)
		tx.EXPECT().Commit().AnyTimes()

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

		// Wait for earlier snapshot process to complete
		time.Sleep(2 * time.Second)

		pub.EXPECT().PublishStateNode(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
		err = service.CreateSnapshot(params)
		if err != nil {
			t.Fatal(err)
		}

		_, err = os.Stat(recovery)
		if err == nil {
			t.Fatal("recovery file still present")
		} else {
			if !os.IsNotExist(err) {
				t.Fatal(err)
			}
		}
	}

	testCases := []int{1, 4, 8, 16, 32}
	for _, tc := range testCases {
		t.Run("case", func(t *testing.T) { runCase(t, tc) })
	}
}
