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
	"github.com/spf13/viper"

	"github.com/vulcanize/ipfs-blockchain-watcher/pkg/config"
	"github.com/vulcanize/ipfs-blockchain-watcher/pkg/core"
)

const (
	ANCIENT_DB_PATH   = "ANCIENT_DB_PATH"
	ETH_CLIENT_NAME   = "ETH_CLIENT_NAME"
	ETH_GENESIS_BLOCK = "ETH_GENESIS_BLOCK"
	ETH_NETWORK_ID    = "ETH_NETWORK_ID"
	ETH_NODE_ID       = "ETH_NODE_ID"
	LVL_DB_PATH       = "LVL_DB_PATH"
)

type Config struct {
	LevelDBPath   string
	AncientDBPath string
	Node          core.Node
	DBConfig      config.Database
}

func (c *Config) Init() {
	c.DBConfig.Init()
	viper.BindEnv("leveldb.path", LVL_DB_PATH)
	viper.BindEnv("ethereum.nodeID", ETH_NODE_ID)
	viper.BindEnv("ethereum.clientName", ETH_CLIENT_NAME)
	viper.BindEnv("ethereum.genesisBlock", ETH_GENESIS_BLOCK)
	viper.BindEnv("ethereum.networkID", ETH_NETWORK_ID)
	viper.BindEnv("leveldb.ancient", ANCIENT_DB_PATH)

	c.Node = core.Node{
		ID:           viper.GetString("ethereum.nodeID"),
		ClientName:   viper.GetString("ethereum.clientName"),
		GenesisBlock: viper.GetString("ethereum.genesisBlock"),
		NetworkID:    viper.GetString("ethereum.networkID"),
	}
	c.LevelDBPath = viper.GetString("leveldb.path")
	c.AncientDBPath = viper.GetString("leveldb.ancient")
}
