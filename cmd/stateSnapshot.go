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
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/vulcanize/eth-pg-ipfs-state-snapshot/pkg/snapshot"
)

// stateSnapshotCmd represents the stateSnapshot command
var stateSnapshotCmd = &cobra.Command{
	Use:   "stateSnapshot",
	Short: "Extract the entire Ethereum state from leveldb and publish into PG-IPFS",
	Long: `Usage

./eth-pg-ipfs-state-snapshot stateSnapshot --config={path to toml config file}`,
	Run: func(cmd *cobra.Command, args []string) {
		subCommand = cmd.CalledAs()
		logWithCommand = *logrus.WithField("SubCommand", subCommand)
		stateSnapshot()
	},
}

func stateSnapshot() {
	serviceConfig := snapshot.ServiceConfig{}
	serviceConfig.Init()
	snapshotService, err := snapshot.NewSnapshotService(serviceConfig)
	if err != nil {
		logWithCommand.Fatal(err)
	}
	height := viper.GetInt64("snapshot.blockHeight")
	workers := viper.GetUint("snapshot.workers")
	params := snapshot.SnapshotParams{Workers: workers}
	if height < 0 {
		if err := snapshotService.CreateLatestSnapshot(params); err != nil {
			logWithCommand.Fatal(err)
		}
	} else {
		params.Height = uint64(height)
		if err := snapshotService.CreateSnapshot(params); err != nil {
			logWithCommand.Fatal(err)
		}
	}
	logWithCommand.Infof("state snapshot at height %d is complete", height)
}

func init() {
	rootCmd.AddCommand(stateSnapshotCmd)

	stateSnapshotCmd.PersistentFlags().String("leveldb-path", "", "path to primary datastore")
	stateSnapshotCmd.PersistentFlags().String("ancient-path", "", "path to ancient datastore")
	stateSnapshotCmd.PersistentFlags().String("block-height", "", "blockheight to extract state at")
	stateSnapshotCmd.PersistentFlags().Int("workers", 0, "number of concurrent workers to use")

	viper.BindPFlag("leveldb.path", stateSnapshotCmd.PersistentFlags().Lookup("leveldb-path"))
	viper.BindPFlag("leveldb.ancient", stateSnapshotCmd.PersistentFlags().Lookup("ancient-path"))
	viper.BindPFlag("snapshot.blockHeight", stateSnapshotCmd.PersistentFlags().Lookup("block-height"))
	viper.BindPFlag("snapshot.workers", stateSnapshotCmd.PersistentFlags().Lookup("workers"))
}
