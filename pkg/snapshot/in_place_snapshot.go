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
	"github.com/jmoiron/sqlx"
	"github.com/sirupsen/logrus"
	. "github.com/cerc-io/ipld-eth-state-snapshot/pkg/types"
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
	db, err := sqlx.Connect("postgres", config.DB.ConnConfig.DbConnectionString())
	if err != nil {
		return err
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}

	defer func() {
		err = CommitOrRollback(tx, err)
		if err != nil {
			logrus.Errorf("CommitOrRollback failed: %s", err)
		}
	}()

	if _, err = tx.Exec(stateSnapShotPgStr, params.StartHeight, params.EndHeight); err != nil {
		return err
	}

	if _, err = tx.Exec(storageSnapShotPgStr, params.StartHeight, params.EndHeight); err != nil {
		return err
	}

	return nil
}
