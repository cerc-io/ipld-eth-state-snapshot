// Copyright © 2022 Vulcanize, Inc
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

package export

import (
	"errors"

	"github.com/spf13/viper"
)

type Config struct {
	ExportLevelDBPath   string
	ExportAncientDBPath string
	ImportLevelDBPath   string
	ImportAncientDBPath string
}

func NewConfig() (*Config, error) {
	conf := new(Config)
	return conf, conf.Init()
}

func (c *Config) Init() error {
	viper.BindEnv(IMPORT_LEVELDB_PATH_TOML, IMPORT_LEVELDB_PATH)
	viper.BindEnv(IMPORT_ANCIENT_PATH_TOML, IMPORT_ANCIENT_PATH)
	viper.BindEnv(EXPORT_LEVELDB_PATH_TOML, EXPORT_LEVELDB_PATH)
	viper.BindEnv(EXPORT_ANCIENT_PATH_TOML, EXPORT_ANCIENT_PATH)

	importLevelDBPath := viper.GetString(IMPORT_LEVELDB_PATH_TOML)
	if importLevelDBPath == "" {
		return errors.New("import levelDB path cannot be empty")
	}
	importAncientPath := viper.GetString(IMPORT_ANCIENT_PATH_TOML)
	if importAncientPath == "" {
		return errors.New("import ancient path cannot be empty")
	}
	exportLevelDBPath := viper.GetString(EXPORT_LEVELDB_PATH_TOML)
	if exportLevelDBPath == "" {
		return errors.New("export levelDB path cannot be empty")
	}
	exportAncientPath := viper.GetString(EXPORT_ANCIENT_PATH_TOML)
	if exportAncientPath == "" {
		return errors.New("export ancient path cannot be empty")
	}
	c.ImportLevelDBPath = importLevelDBPath
	c.ImportAncientDBPath = importAncientPath
	c.ExportLevelDBPath = exportLevelDBPath
	c.ExportAncientDBPath = exportAncientPath
	return nil
}
