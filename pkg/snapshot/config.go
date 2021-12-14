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
	ethNode "github.com/ethereum/go-ethereum/statediff/indexer/node"
	"github.com/ethereum/go-ethereum/statediff/indexer/postgres"
	"github.com/spf13/viper"
)

const (
	ancientDBPath   = "ANCIENT_DB_PATH"
	ethClientName   = "ETH_CLIENT_NAME"
	ethGenesisBlock = "ETH_GENESIS_BLOCK"
	ethNetworkID    = "ETH_NETWORK_ID"
	ethNodeID       = "ETH_NODE_ID"
	lvlDBPath       = "LVL_DB_PATH"
)

// Config is config parameters for DB.
type Config struct {
	LevelDBPath   string
	AncientDBPath string
	Node          ethNode.Info
	connectionURI string
	DBConfig      postgres.ConnectionConfig
}

// Init Initialises config
func (c *Config) Init() {
	c.dbInit()
	viper.BindEnv("leveldb.path", lvlDBPath)
	viper.BindEnv("ethereum.nodeID", ethNodeID)
	viper.BindEnv("ethereum.clientName", ethClientName)
	viper.BindEnv("ethereum.genesisBlock", ethGenesisBlock)
	viper.BindEnv("ethereum.networkID", ethNetworkID)
	viper.BindEnv("leveldb.ancient", ancientDBPath)

	c.Node = ethNode.Info{
		ID:           viper.GetString("ethereum.nodeID"),
		ClientName:   viper.GetString("ethereum.clientName"),
		GenesisBlock: viper.GetString("ethereum.genesisBlock"),
		NetworkID:    viper.GetString("ethereum.networkID"),
		ChainID:      viper.GetUint64("ethereum.chainID"),
	}
	c.LevelDBPath = viper.GetString("leveldb.path")
	c.AncientDBPath = viper.GetString("leveldb.ancient")
}

func (c *Config) dbInit() {
	viper.BindEnv("database.name", postgres.DATABASE_NAME)
	viper.BindEnv("database.hostname", postgres.DATABASE_HOSTNAME)
	viper.BindEnv("database.port", postgres.DATABASE_PORT)
	viper.BindEnv("database.user", postgres.DATABASE_USER)
	viper.BindEnv("database.password", postgres.DATABASE_PASSWORD)
	viper.BindEnv("database.maxIdle", postgres.DATABASE_MAX_IDLE_CONNECTIONS)
	viper.BindEnv("database.maxOpen", postgres.DATABASE_MAX_OPEN_CONNECTIONS)
	viper.BindEnv("database.maxLifetime", postgres.DATABASE_MAX_CONN_LIFETIME)

	dbParams := postgres.ConnectionParams{}
	// DB params
	dbParams.Name = viper.GetString("database.name")
	dbParams.Hostname = viper.GetString("database.hostname")
	dbParams.Port = viper.GetInt("database.port")
	dbParams.User = viper.GetString("database.user")
	dbParams.Password = viper.GetString("database.password")

	c.connectionURI = postgres.DbConnectionString(dbParams)
	// DB config
	c.DBConfig.MaxIdle = viper.GetInt("database.maxIdle")
	c.DBConfig.MaxOpen = viper.GetInt("database.maxOpen")
	c.DBConfig.MaxLifetime = viper.GetInt("database.maxLifetime")
}
