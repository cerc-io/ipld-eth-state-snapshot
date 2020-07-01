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
	"github.com/ethereum/go-ethereum/common"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/vulcanize/eth-pg-ipfs-state-snapshot/pkg/snapshot"
)

// stateSnapshotCmd represents the stateSnapshot command
var stateSnapshotCmd = &cobra.Command{
	Use:   "stateSnapshot",
	Short: "Extract the entire Ethereum state from leveldb and publish into PG-IPFS",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		subCommand = cmd.CalledAs()
		logWithCommand = *logrus.WithField("SubCommand", subCommand)
		stateSnapshot()
	},
}

func stateSnapshot() {
	snapConfig := snapshot.Config{}
	snapConfig.Init()
	snapshotService, err := snapshot.NewSnapshotService(snapConfig)
	if err != nil {
		logWithCommand.Fatal(err)
	}
	height := viper.Get("snapshot.blockHeight")
	uHeight, ok := height.(uint64)
	if !ok {
		logWithCommand.Fatal("snapshot.blockHeight needs to be a uint")
	}
	hashStr := viper.GetString("snapshot.blockHash")
	hash := common.HexToHash(hashStr)
	if err := snapshotService.CreateSnapshot(uHeight, hash); err != nil {
		logWithCommand.Fatal(err)
	}
	logWithCommand.Infof("state snapshot for height %d and hash %s is complete", uHeight, hashStr)
}

func init() {
	rootCmd.AddCommand(stateSnapshotCmd)

	stateSnapshotCmd.PersistentFlags().String("leveldb-path", "", "path to leveldb")
	stateSnapshotCmd.PersistentFlags().String("block-height", "", "blockheight to extract state at")
	stateSnapshotCmd.PersistentFlags().String("block-hash", "", "blockhash to extract state at")

	viper.BindPFlag("leveldb.path", stateSnapshotCmd.PersistentFlags().Lookup("leveldb-path"))
	viper.BindPFlag("snapshot.blockHeight", stateSnapshotCmd.PersistentFlags().Lookup("block-height"))
	viper.BindPFlag("snapshot.blockHash", stateSnapshotCmd.PersistentFlags().Lookup("block-hash"))
}
