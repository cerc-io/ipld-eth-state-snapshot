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
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"github.com/sirupsen/logrus"

	"github.com/ethereum/go-ethereum/statediff/indexer/database/sql/postgres"
	ethNode "github.com/ethereum/go-ethereum/statediff/indexer/node"
	"github.com/spf13/viper"
)

// SnapshotMode specifies the snapshot data output method
type SnapshotMode string

const (
	PgSnapshot   SnapshotMode = "postgres"
	FileSnapshot SnapshotMode = "file"

	defaultOutputDir = "./snapshot_output"
)

// Config contains params for both databases the service uses
type Config struct {
	Eth     *EthConfig
	DB      *DBConfig
	File    *FileConfig
	Service *ServiceConfig
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

type ServiceConfig struct {
	AllowedPaths    [][]byte
	AllowedAccounts []common.Address
}

func NewConfig(mode SnapshotMode) (*Config, error) {
	ret := &Config{
		&EthConfig{},
		&DBConfig{},
		&FileConfig{},
		&ServiceConfig{},
	}
	return ret, ret.Init(mode)
}

func NewInPlaceSnapshotConfig() *Config {
	ret := &Config{
		&EthConfig{},
		&DBConfig{},
		&FileConfig{},
	}
	ret.DB.Init()

	return ret
}

// Init Initialises config
func (c *Config) Init(mode SnapshotMode) error {
	viper.BindEnv(ETH_NODE_ID_TOML, ETH_NODE_ID)
	viper.BindEnv(ETH_CLIENT_NAME_TOML, ETH_CLIENT_NAME)
	viper.BindEnv(ETH_GENESIS_BLOCK_TOML, ETH_GENESIS_BLOCK)
	viper.BindEnv(ETH_NETWORK_ID_TOML, ETH_NETWORK_ID)
	viper.BindEnv(ETH_CHAIN_ID_TOML, ETH_CHAIN_ID)

	c.Eth.NodeInfo = ethNode.Info{
		ID:           viper.GetString(ETH_NODE_ID_TOML),
		ClientName:   viper.GetString(ETH_CLIENT_NAME_TOML),
		GenesisBlock: viper.GetString(ETH_GENESIS_BLOCK_TOML),
		NetworkID:    viper.GetString(ETH_NETWORK_ID_TOML),
		ChainID:      viper.GetUint64(ETH_CHAIN_ID_TOML),
	}

	viper.BindEnv(ANCIENT_DB_PATH_TOML, ANCIENT_DB_PATH)
	viper.BindEnv(LVL_DB_PATH_TOML, LVL_DB_PATH)

	c.Eth.AncientDBPath = viper.GetString(ANCIENT_DB_PATH_TOML)
	c.Eth.LevelDBPath = viper.GetString(LVL_DB_PATH_TOML)

	switch mode {
	case FileSnapshot:
		c.File.Init()
	case PgSnapshot:
		c.DB.Init()
	default:
		return fmt.Errorf("no output mode specified")
	}
	return c.Service.Init()
}

func (c *DBConfig) Init() {
	viper.BindEnv(DATABASE_NAME_TOML, DATABASE_NAME)
	viper.BindEnv(DATABASE_HOSTNAME_TOML, DATABASE_HOSTNAME)
	viper.BindEnv(DATABASE_PORT_TOML, DATABASE_PORT)
	viper.BindEnv(DATABASE_USER_TOML, DATABASE_USER)
	viper.BindEnv(DATABASE_PASSWORD_TOML, DATABASE_PASSWORD)
	viper.BindEnv(DATABASE_MAX_IDLE_CONNECTIONS_TOML, DATABASE_MAX_IDLE_CONNECTIONS)
	viper.BindEnv(DATABASE_MAX_OPEN_CONNECTIONS_TOML, DATABASE_MAX_OPEN_CONNECTIONS)
	viper.BindEnv(DATABASE_MAX_CONN_LIFETIME_TOML, DATABASE_MAX_CONN_LIFETIME)

	dbParams := postgres.Config{}
	// DB params
	dbParams.DatabaseName = viper.GetString(DATABASE_NAME_TOML)
	dbParams.Hostname = viper.GetString(DATABASE_HOSTNAME_TOML)
	dbParams.Port = viper.GetInt(DATABASE_PORT_TOML)
	dbParams.Username = viper.GetString(DATABASE_USER_TOML)
	dbParams.Password = viper.GetString(DATABASE_PASSWORD_TOML)
	// Connection config
	dbParams.MaxIdle = viper.GetInt(DATABASE_MAX_IDLE_CONNECTIONS_TOML)
	dbParams.MaxConns = viper.GetInt(DATABASE_MAX_OPEN_CONNECTIONS_TOML)
	dbParams.MaxConnLifetime = time.Duration(viper.GetInt(DATABASE_MAX_CONN_LIFETIME_TOML)) * time.Second

	c.ConnConfig = dbParams
	c.URI = dbParams.DbConnectionString()
}

func (c *FileConfig) Init() error {
	viper.BindEnv(FILE_OUTPUT_DIR_TOML, FILE_OUTPUT_DIR)
	c.OutputDir = viper.GetString(FILE_OUTPUT_DIR_TOML)
	if c.OutputDir == "" {
		logrus.Infof("no output directory set, using default: %s", defaultOutputDir)
		c.OutputDir = defaultOutputDir
	}
	return nil
}

func (c *ServiceConfig) Init() error {
	viper.BindEnv(SNAPSHOT_ACCOUNTS_TOML, SNAPSHOT_ACCOUNTS)
	allowedAccounts := viper.GetStringSlice(SNAPSHOT_ACCOUNTS)
	accountsLen := len(allowedAccounts)
	if accountsLen != 0 {
		c.AllowedAccounts = make([]common.Address, accountsLen)
		for i, allowedAccount := range allowedAccounts {
			c.AllowedAccounts[i] = common.HexToAddress(allowedAccount)
		}
	} else {
		logrus.Infof("no snapshot addresses specified, will perform snapshot of entire trie(s)")
	}
	return nil
}
