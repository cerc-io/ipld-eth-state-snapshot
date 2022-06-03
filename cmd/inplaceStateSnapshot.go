// VulcanizeDB
// Copyright © 2022 Vulcanize

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.

// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package cmd

import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/vulcanize/ipld-eth-state-snapshot/pkg/snapshot"
)

// inPlaceStateSnapshotCmd represents the inPlaceStateSnapshot command
var inPlaceStateSnapshotCmd = &cobra.Command{
	Use:   "inPlaceStateSnapshot",
	Short: "Take an in-place state snapshot in the database",
	Long: `Usage:

	./ipld-eth-state-snapshot inPlaceStateSnapshot --config={path to toml config file}`,
	Run: func(cmd *cobra.Command, args []string) {
		subCommand = cmd.CalledAs()
		logWithCommand = *logrus.WithField("SubCommand", subCommand)
		inPlaceStateSnapshot()
	},
}

func inPlaceStateSnapshot() {
	config := snapshot.NewInPlaceSnapshotConfig()

	startHeight := viper.GetUint64(snapshot.SNAPSHOT_START_HEIGHT_TOML)
	endHeight := viper.GetUint64(snapshot.SNAPSHOT_END_HEIGHT_TOML)

	params := snapshot.InPlaceSnapshotParams{StartHeight: uint64(startHeight), EndHeight: uint64(endHeight)}
	if err := snapshot.CreateInPlaceSnapshot(config, params); err != nil {
		logWithCommand.Fatal(err)
	}

	logWithCommand.Infof("snapshot taken at height %d starting from height %d", endHeight, startHeight)
}

func init() {
	rootCmd.AddCommand(inPlaceStateSnapshotCmd)

	stateSnapshotCmd.PersistentFlags().String(snapshot.SNAPSHOT_START_HEIGHT_CLI, "", "start block height for in-place snapshot")
	stateSnapshotCmd.PersistentFlags().String(snapshot.SNAPSHOT_END_HEIGHT_CLI, "", "end block height for in-place snapshot")

	viper.BindPFlag(snapshot.SNAPSHOT_START_HEIGHT_TOML, stateSnapshotCmd.PersistentFlags().Lookup(snapshot.SNAPSHOT_START_HEIGHT_CLI))
	viper.BindPFlag(snapshot.SNAPSHOT_END_HEIGHT_TOML, stateSnapshotCmd.PersistentFlags().Lookup(snapshot.SNAPSHOT_END_HEIGHT_CLI))
}
