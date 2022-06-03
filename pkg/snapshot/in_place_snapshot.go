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

package snapshot

import (
	"context"

	"github.com/ethereum/go-ethereum/statediff/indexer/database/sql/postgres"
)

const (
	stateSnapShotPgStr   = "SELECT state_snapshot($1, $2)"
	storageSnapShotPgStr = "SELECT storage_snapshot($1, $2)"
)

type InPlaceSnapshotParams struct {
	StartHeight uint64
	EndHeight   uint64
}

func CreateInPlaceSnapshot(config *Config, params InPlaceSnapshotParams) error {
	ctx := context.Background()
	driver, err := postgres.NewPGXDriver(ctx, config.DB.ConnConfig, config.Eth.NodeInfo)
	if err != nil {
		return err
	}
	db := postgres.NewPostgresDB(driver)

	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}

	tx.Exec(ctx, stateSnapShotPgStr, params.StartHeight, params.EndHeight)
	tx.Exec(ctx, storageSnapShotPgStr, params.StartHeight, params.EndHeight)

	err = tx.Commit(ctx)
	if err != nil {
		return err
	}

	return nil
}
