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
