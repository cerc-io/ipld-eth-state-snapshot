package snapshot

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/golang/mock/gomock"

	ethNode "github.com/ethereum/go-ethereum/statediff/indexer/node"
	"github.com/ethereum/go-ethereum/statediff/indexer/postgres"

	fixt "github.com/vulcanize/eth-pg-ipfs-state-snapshot/fixture"
	"github.com/vulcanize/eth-pg-ipfs-state-snapshot/pkg/snapshot/mock"
)

func testConfig(leveldbpath, ancientdbpath string) *Config {
	dbParams := postgres.ConnectionParams{
		Name:     "snapshot_test",
		Hostname: "localhost",
		Port:     5432,
		User:     "tester",
		Password: "test_pw",
	}
	connconfig := postgres.ConnectionConfig{
		MaxIdle:     0,
		MaxLifetime: 0,
		MaxOpen:     4,
	}
	nodeinfo := ethNode.Info{
		ID:           "eth_node_id",
		ClientName:   "eth_client",
		GenesisBlock: "X",
		NetworkID:    "eth_network",
		ChainID:      0,
	}

	return &Config{
		DB: &DBConfig{
			Node:       nodeinfo,
			URI:        postgres.DbConnectionString(dbParams),
			ConnConfig: connconfig,
		},
		Eth: &EthConfig{
			LevelDBPath:   leveldbpath,
			AncientDBPath: ancientdbpath,
		},
	}
}

func TestCreateSnapshot(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	fixture_path := filepath.Join(wd, "..", "..", "fixture")
	datadir := filepath.Join(fixture_path, "chaindata")
	if _, err := os.Stat(datadir); err != nil {
		t.Fatal("no chaindata:", err)
	}
	config := testConfig(datadir, filepath.Join(datadir, "ancient"))
	fmt.Printf("config: %+v %+v\n", config.DB, config.Eth)

	edb, err := NewLevelDB(config.Eth)
	if err != nil {
		t.Fatal(err)
	}
	workers := 8

	pub := mock.NewMockPublisher(t)
	pub.EXPECT().PublishHeader(gomock.Eq(fixt.PublishHeader))
	pub.EXPECT().BeginTx().
		Times(workers)
	pub.EXPECT().PrepareTxForBatch(gomock.Any(), gomock.Any()).
		Times(workers)
	pub.EXPECT().PublishStateNode(gomock.Any(), mock.AnyOf(int64(0), int64(1)), gomock.Any()).
		Times(workers)
	// TODO: fixtures for storage node
	// pub.EXPECT().PublishStorageNode(gomock.Eq(fixt.StorageNode), gomock.Eq(int64(0)), gomock.Any())
	pub.EXPECT().CommitTx(gomock.Any()).
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
	// 	t.Fatal(err)
	// }
}
