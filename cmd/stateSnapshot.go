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

package cmd

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/vulcanize/ipld-eth-state-snapshot/pkg/snapshot"
)

// stateSnapshotCmd represents the stateSnapshot command
var stateSnapshotCmd = &cobra.Command{
	Use:   "stateSnapshot",
	Short: "Extract the entire Ethereum state from leveldb and publish into PG-IPFS",
	Long: `Usage

./ipld-eth-state-snapshot stateSnapshot --config={path to toml config file}`,
	Run: func(cmd *cobra.Command, args []string) {
		subCommand = cmd.CalledAs()
		logWithCommand = *logrus.WithField("SubCommand", subCommand)
		stateSnapshot()
	},
}

func stateSnapshot() {
	modeStr := viper.GetString(snapshot.SNAPSHOT_MODE_TOML)
	mode := snapshot.SnapshotMode(modeStr)
	config, err := snapshot.NewConfig(mode)
	if err != nil {
		logWithCommand.Fatalf("unable to initialize config: %v", err)
	}
	logWithCommand.Infof("opening levelDB and ancient data at %s and %s",
		config.Eth.LevelDBPath, config.Eth.AncientDBPath)
	edb, err := snapshot.NewLevelDB(config.Eth)
	if err != nil {
		logWithCommand.Fatal(err)
	}
	height := viper.GetInt64(snapshot.SNAPSHOT_BLOCK_HEIGHT_TOML)
	recoveryFile := viper.GetString(snapshot.SNAPSHOT_RECOVERY_FILE_TOML)
	if recoveryFile == "" {
		recoveryFile = fmt.Sprintf("./%d_snapshot_recovery", height)
		logWithCommand.Infof("no recovery file set, using default: %s", recoveryFile)
	}

	pub, err := snapshot.NewPublisher(mode, config)
	if err != nil {
		logWithCommand.Fatal(err)
	}

	snapshotService, err := snapshot.NewSnapshotService(edb, pub, recoveryFile)
	if err != nil {
		logWithCommand.Fatal(err)
	}
	workers := viper.GetUint(snapshot.SNAPSHOT_WORKERS_TOML)
	if height < 0 {
		if err := snapshotService.CreateLatestSnapshot(workers, config.Service.AllowedAccounts); err != nil {
			logWithCommand.Fatal(err)
		}
	} else {
		params := snapshot.SnapshotParams{Workers: workers, Height: uint64(height), WatchedAddresses: config.Service.AllowedAccounts}
		if err := snapshotService.CreateSnapshot(params); err != nil {
			logWithCommand.Fatal(err)
		}
	}
	logWithCommand.Infof("state snapshot at height %d is complete", height)
}

func init() {
	rootCmd.AddCommand(stateSnapshotCmd)

	stateSnapshotCmd.PersistentFlags().String(snapshot.LVL_DB_PATH_CLI, "", "path to primary datastore")
	stateSnapshotCmd.PersistentFlags().String(snapshot.ANCIENT_DB_PATH_CLI, "", "path to ancient datastore")
	stateSnapshotCmd.PersistentFlags().String(snapshot.SNAPSHOT_BLOCK_HEIGHT_CLI, "", "block height to extract state at")
	stateSnapshotCmd.PersistentFlags().Int(snapshot.SNAPSHOT_WORKERS_CLI, 1, "number of concurrent workers to use")
	stateSnapshotCmd.PersistentFlags().String(snapshot.SNAPSHOT_RECOVERY_FILE_CLI, "", "file to recover from a previous iteration")
	stateSnapshotCmd.PersistentFlags().String(snapshot.SNAPSHOT_MODE_CLI, "postgres", "output mode for snapshot ('file' or 'postgres')")
	stateSnapshotCmd.PersistentFlags().String(snapshot.FILE_OUTPUT_DIR_CLI, "", "directory for writing ouput to while operating in 'file' mode")
	stateSnapshotCmd.PersistentFlags().StringArray(snapshot.SNAPSHOT_ACCOUNTS_CLI, nil, "list of account addresses to limit snapshot to")

	viper.BindPFlag(snapshot.LVL_DB_PATH_TOML, stateSnapshotCmd.PersistentFlags().Lookup(snapshot.LVL_DB_PATH_CLI))
	viper.BindPFlag(snapshot.ANCIENT_DB_PATH_TOML, stateSnapshotCmd.PersistentFlags().Lookup(snapshot.ANCIENT_DB_PATH_CLI))
	viper.BindPFlag(snapshot.SNAPSHOT_BLOCK_HEIGHT_TOML, stateSnapshotCmd.PersistentFlags().Lookup(snapshot.SNAPSHOT_BLOCK_HEIGHT_CLI))
	viper.BindPFlag(snapshot.SNAPSHOT_WORKERS_TOML, stateSnapshotCmd.PersistentFlags().Lookup(snapshot.SNAPSHOT_WORKERS_CLI))
	viper.BindPFlag(snapshot.SNAPSHOT_RECOVERY_FILE_TOML, stateSnapshotCmd.PersistentFlags().Lookup(snapshot.SNAPSHOT_RECOVERY_FILE_CLI))
	viper.BindPFlag(snapshot.SNAPSHOT_MODE_TOML, stateSnapshotCmd.PersistentFlags().Lookup(snapshot.SNAPSHOT_MODE_CLI))
	viper.BindPFlag(snapshot.FILE_OUTPUT_DIR_TOML, stateSnapshotCmd.PersistentFlags().Lookup(snapshot.FILE_OUTPUT_DIR_CLI))
	viper.BindPFlag(snapshot.SNAPSHOT_ACCOUNTS_TOML, stateSnapshotCmd.PersistentFlags().Lookup(snapshot.SNAPSHOT_ACCOUNTS_CLI))
}
