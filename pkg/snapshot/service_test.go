package snapshot

import (
	"testing"

	"github.com/golang/mock/gomock"

	fixt "github.com/vulcanize/eth-pg-ipfs-state-snapshot/fixture"
	mock "github.com/vulcanize/eth-pg-ipfs-state-snapshot/mocks/snapshot"
	"github.com/vulcanize/eth-pg-ipfs-state-snapshot/test"
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

func TestCreateSnapshot(t *testing.T) {
	config := testConfig(fixt.ChaindataPath, fixt.AncientdataPath)

	edb, err := NewLevelDB(config.Eth)
	if err != nil {
		t.Fatal(err)
	}
	workers := 4

	ctl := gomock.NewController(t)
	tx := mock.NewMockTx(ctl)
	pub := mock.NewMockPublisher(ctl)

	pub.EXPECT().PublishHeader(gomock.Eq(&fixt.Header1))
	pub.EXPECT().BeginTx().
		Return(tx, nil).
		Times(workers)
	pub.EXPECT().PrepareTxForBatch(gomock.Any(), gomock.Any()).
		Return(tx, nil).
		Times(workers)
	pub.EXPECT().PublishStateNode(gomock.Any(), gomock.Any(), gomock.Any()).
		Times(workers)
	// TODO: fixtures for storage node
	// pub.EXPECT().PublishStorageNode(gomock.Eq(fixt.StorageNode), gomock.Eq(int64(0)), gomock.Any())
	// pub.EXPECT().CommitTx(gomock.Any()).
	//	Times(workers)

	tx.EXPECT().Commit().
		Times(workers)

	service, err := NewSnapshotService(edb, pub)
	if err != nil {
		t.Fatal(err)
	}

	params := SnapshotParams{Height: 1, Workers: uint(workers)}
	err = service.CreateSnapshot(params)
	if err != nil {
		t.Fatal(err)
	}

	// err = service.CreateLatestSnapshot(0)
	// if err != nil {
	//	t.Fatal(err)
	// }
}
