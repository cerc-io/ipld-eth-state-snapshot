package snapshot

import (
	"path/filepath"
	"testing"

	ethNode "github.com/ethereum/go-ethereum/statediff/indexer/node"
	"github.com/ethereum/go-ethereum/statediff/indexer/postgres"
)

func testConfig(leveldbpath, ancientdbpath string) *Config {
	dbParams := postgres.ConnectionParams{}
	dbParams.Name = "snapshot_test"
	dbParams.Hostname = "localhost"
	dbParams.Port = 5432
	dbParams.User = "tester"
	dbParams.Password = "test_pw"
	uri := postgres.DbConnectionString(dbParams)
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
			URI:        uri,
			ConnConfig: connconfig,
		},
		Eth: &EthConfig{
			LevelDBPath:   leveldbpath,
			AncientDBPath: ancientdbpath,
		},
	}
}

func NewMockPublisher() *Publisher {
	return nil
}

func TestCreateSnapshot(t *testing.T) {
	datadir := t.TempDir()
	config := testConfig(
		filepath.Join(datadir, "leveldb"),
		filepath.Join(datadir, "ancient"),
	)

	pub := NewMockPublisher()
	service, err := NewSnapshotService(config.Eth, pub)
	if err != nil {
		t.Fatal(err)
	}

	params := SnapshotParams{Height: 1}
	err = service.CreateSnapshot(params)
	if err != nil {
		t.Fatal(err)
	}

	// err = service.CreateLatestSnapshot(0)
	// if err != nil {
	// 	t.Fatal(err)
	// }
}
