package snapshot

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/statediff/indexer/database/sql/postgres"

	"github.com/vulcanize/ipld-eth-state-snapshot/pkg/prom"
	file "github.com/vulcanize/ipld-eth-state-snapshot/pkg/snapshot/file"
	pg "github.com/vulcanize/ipld-eth-state-snapshot/pkg/snapshot/pg"
	snapt "github.com/vulcanize/ipld-eth-state-snapshot/pkg/types"
)

func NewPublisher(mode SnapshotMode, config *Config) (snapt.Publisher, error) {
	switch mode {
	case PgSnapshot:
		driver, err := postgres.NewPGXDriver(context.Background(), config.DB.ConnConfig, config.Eth.NodeInfo)
		if err != nil {
			return nil, err
		}

		prom.RegisterDBCollector(config.DB.ConnConfig.DatabaseName, driver)

		return pg.NewPublisher(postgres.NewPostgresDB(driver)), nil
	case FileSnapshot:
		return file.NewPublisher(config.File.OutputDir, config.Eth.NodeInfo)
	}
	return nil, fmt.Errorf("invalid snapshot mode: %s", mode)
}

// Subtracts 1 from the last byte in a path slice, carrying if needed.
// Does nothing, returning false, for all-zero inputs.
func decrementPath(path []byte) bool {
	// check for all zeros
	allzero := true
	for i := 0; i < len(path); i++ {
		allzero = allzero && path[i] == 0
	}
	if allzero {
		return false
	}
	for i := len(path) - 1; i >= 0; i-- {
		val := path[i]
		path[i]--
		if val == 0 {
			path[i] = 0xf
		} else {
			return true
		}
	}
	return true
}

// https://github.com/ethereum/go-ethereum/blob/master/trie/encoding.go#L97
func keybytesToHex(str []byte) []byte {
	l := len(str)*2 + 1
	var nibbles = make([]byte, l)
	for i, b := range str {
		nibbles[i*2] = b / 16
		nibbles[i*2+1] = b % 16
	}
	nibbles[l-1] = 16
	return nibbles
}
