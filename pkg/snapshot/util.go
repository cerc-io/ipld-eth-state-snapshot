package snapshot

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/statediff/indexer/database/sql/postgres"

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
		return pg.NewPublisher(postgres.NewPostgresDB(driver)), nil
	case FileSnapshot:
		return file.NewPublisher(config.File.OutputDir, config.Eth.NodeInfo)
	}
	return nil, fmt.Errorf("invalid snapshot mode: %s", mode)
}
