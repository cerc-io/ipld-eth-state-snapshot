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

package snapshot

import (
	"time"

	"github.com/ethereum/go-ethereum/statediff/indexer/database/sql/postgres"
	ethNode "github.com/ethereum/go-ethereum/statediff/indexer/node"
	"github.com/spf13/viper"
)

const (
	ANCIENT_DB_PATH   = "ANCIENT_DB_PATH"
	ETH_CLIENT_NAME   = "ETH_CLIENT_NAME"
	ETH_GENESIS_BLOCK = "ETH_GENESIS_BLOCK"
	ETH_NETWORK_ID    = "ETH_NETWORK_ID"
	ETH_NODE_ID       = "ETH_NODE_ID"
	LVL_DB_PATH       = "LVL_DB_PATH"
)

// SnapshotMode specifies the snapshot data output method
type SnapshotMode string

const (
	PgSnapshot   SnapshotMode = "postgres"
	FileSnapshot SnapshotMode = "file"
)

// Config contains params for both databases the service uses
type Config struct {
	Eth  *EthConfig
	DB   *DBConfig
	File *FileConfig
}

// EthConfig is config parameters for the chain.
type EthConfig struct {
	LevelDBPath   string
	AncientDBPath string
	NodeInfo      ethNode.Info
}

// DBConfig is config parameters for DB.
type DBConfig struct {
	URI        string
	ConnConfig postgres.Config
}

type FileConfig struct {
	OutputDir string
}

func NewConfig() *Config {
	ret := &Config{
		&EthConfig{},
		&DBConfig{},
		&FileConfig{},
	}
	ret.Init()
	return ret
}

// Init Initialises config
func (c *Config) Init() {
	viper.BindEnv("ethereum.nodeID", ETH_NODE_ID)
	viper.BindEnv("ethereum.clientName", ETH_CLIENT_NAME)
	viper.BindEnv("ethereum.genesisBlock", ETH_GENESIS_BLOCK)
	viper.BindEnv("ethereum.networkID", ETH_NETWORK_ID)

	c.Eth.NodeInfo = ethNode.Info{
		ID:           viper.GetString("ethereum.nodeID"),
		ClientName:   viper.GetString("ethereum.clientName"),
		GenesisBlock: viper.GetString("ethereum.genesisBlock"),
		NetworkID:    viper.GetString("ethereum.networkID"),
		ChainID:      viper.GetUint64("ethereum.chainID"),
	}

	viper.BindEnv("leveldb.ancient", ANCIENT_DB_PATH)
	viper.BindEnv("leveldb.path", LVL_DB_PATH)

	c.Eth.AncientDBPath = viper.GetString("leveldb.ancient")
	c.Eth.LevelDBPath = viper.GetString("leveldb.path")

	c.DB.Init()
	c.File.Init()
}

func (c *DBConfig) Init() {
	viper.BindEnv("database.name", "DATABASE_NAME")
	viper.BindEnv("database.hostname", "DATABASE_HOSTNAME")
	viper.BindEnv("database.port", "DATABASE_PORT")
	viper.BindEnv("database.user", "DATABASE_USER")
	viper.BindEnv("database.password", "DATABASE_PASSWORD")
	viper.BindEnv("database.maxIdle", "DATABASE_MAX_IDLE_CONNECTIONS")
	viper.BindEnv("database.maxOpen", "DATABASE_MAX_OPEN_CONNECTIONS")
	viper.BindEnv("database.maxLifetime", "DATABASE_MAX_CONN_LIFETIME")

	dbParams := postgres.Config{}
	// DB params
	dbParams.DatabaseName = viper.GetString("database.name")
	dbParams.Hostname = viper.GetString("database.hostname")
	dbParams.Port = viper.GetInt("database.port")
	dbParams.Username = viper.GetString("database.user")
	dbParams.Password = viper.GetString("database.password")
	// Connection config
	dbParams.MaxIdle = viper.GetInt("database.maxIdle")
	dbParams.MaxConns = viper.GetInt("database.maxOpen")
	dbParams.MaxConnLifetime = time.Duration(viper.GetInt("database.maxLifetime")) * time.Second

	c.ConnConfig = dbParams
	c.URI = dbParams.DbConnectionString()
}

func (c *FileConfig) Init() {
	viper.BindEnv("file.outputDir", "FILE_OUTPUT_DIR")
	c.OutputDir = viper.GetString("file.outputDir")
}
