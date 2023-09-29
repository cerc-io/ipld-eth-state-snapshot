package snapshot_test

import (
	"github.com/cerc-io/plugeth-statediff/indexer/database/sql/postgres"
	ethnode "github.com/cerc-io/plugeth-statediff/indexer/node"
)

var (
	DefaultNodeInfo = ethnode.Info{
		ID:           "test_nodeid",
		ClientName:   "test_client",
		GenesisBlock: "TEST_GENESIS",
		NetworkID:    "test_network",
		ChainID:      0,
	}
	DefaultPgConfig = postgres.Config{
		Hostname:     "localhost",
		Port:         8077,
		DatabaseName: "cerc_testing",
		Username:     "vdbm",
		Password:     "password",

		MaxIdle:         0,
		MaxConnLifetime: 0,
		MaxConns:        4,
	}
)
