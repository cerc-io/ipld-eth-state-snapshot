package snapshot

import (
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"testing"

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
			Times(len(fixt.Block1_StateNodePaths))

		// TODO: fixtures for storage node
		// pub.EXPECT().PublishStorageNode(gomock.Eq(fixt.StorageNode), gomock.Eq(int64(0)), gomock.Any())

		tx.EXPECT().Commit().
			Times(workers)

		config := testConfig(fixt.ChaindataPath, fixt.AncientdataPath)
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

		config := testConfig(fixt.ChaindataPath, fixt.AncientdataPath)
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

	testCases := []int{1, 4, 32}
	for _, tc := range testCases {
		t.Run("case", func(t *testing.T) { runCase(t, tc) })
	}

}
