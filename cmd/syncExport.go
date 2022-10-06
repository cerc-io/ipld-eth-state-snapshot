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

package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/cerc-io/ipld-eth-state-snapshot/pkg/export"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// stateSnapshotCmd represents the stateSnapshot command
var syncExportCmd = &cobra.Command{
	Use:   "syncExport",
	Short: "Export the state and block data necessary to begin a full sync at the provided block height",
	Long: `Usage

./ipld-eth-state-snapshot syncExport --config={path to toml config file}`,
	Run: func(cmd *cobra.Command, args []string) {
		subCommand = cmd.CalledAs()
		logWithCommand = *logrus.WithField("SubCommand", subCommand)
		syncExport()
	},
}

func syncExport() {
	config, err := export.NewConfig()
	if err != nil {
		logWithCommand.Fatalf("unable to initialize config: %v", err)
	}
	logWithCommand.Infof("opening export and import levelDB and ancient data at %s, %s and %s, %s",
		config.ExportLevelDBPath, config.ExportAncientDBPath, config.ImportLevelDBPath, config.ImportAncientDBPath)
	exportDB, importDB, err := export.OpenLevelDBs(config)
	if err != nil {
		logWithCommand.Fatal(err)
	}
	viper.BindEnv(export.SYNC_EXPORT_HEIGHT_TOML, export.SYNC_EXPORT_HEIGHT)
	height := viper.GetUint64(export.SYNC_EXPORT_HEIGHT_TOML)
	viper.BindEnv(export.SYNC_EXPORT_RECOVERY_FILE_TOML, export.SYNC_EXPORT_RECOVERY_FILE)
	recoveryFile := viper.GetString(export.SYNC_EXPORT_RECOVERY_FILE_TOML)
	if recoveryFile == "" {
		recoveryFile = fmt.Sprintf("./%d_snapshot_recovery", height)
		logWithCommand.Infof("no recovery file set, using default: %s", recoveryFile)
	}

	service, err := export.NewExportService(exportDB, importDB, recoveryFile)
	if err != nil {
		logWithCommand.Fatal(err)
	}

	viper.BindEnv(export.SYNC_EXPORT_SEGMENT_SIZE_TOML, export.SYNC_EXPORT_SEGMENT_SIZE)
	segmentSize := viper.GetUint64(export.SYNC_EXPORT_SEGMENT_SIZE_TOML)

	viper.BindEnv(export.SYNC_EXPORT_TRIE_WORKERS_TOML, export.SYNC_EXPORT_TRIE_WORKERS)
	workers := viper.GetUint(export.SYNC_EXPORT_TRIE_WORKERS_TOML)
	params := export.Params{TrieWorkers: workers, Height: height, SegmentSize: segmentSize}
	ctx, cancelFunc := context.WithCancel(context.Background())
	wg := new(sync.WaitGroup)
	errChan := service.Export(ctx, wg, params)
	go func() {
		for {
			select {
			case err := <-errChan:
				if err != nil {
					logWithCommand.Errorf("error received: %s\r\ncanceling processes", err.Error())
					cancelFunc()
				}
			case <-ctx.Done():
				if err := ctx.Err(); err != nil {
					logWithCommand.Errorf("error collected on cancelation: %s", err.Error())
				}
				return
			}
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		logWithCommand.Infof("signal received (%v), stopping sync export", sig)
		cancelFunc()
	}()
	wg.Wait()
	logWithCommand.Infof("sync export at height %d is complete", height)
}

func init() {
	rootCmd.AddCommand(syncExportCmd)

	stateSnapshotCmd.PersistentFlags().String(export.EXPORT_LEVELDB_PATH_CLI, "", "path to export leveldb")
	stateSnapshotCmd.PersistentFlags().String(export.EXPORT_ANCIENT_PATH_CLI, "", "path to export ancient datastore")
	stateSnapshotCmd.PersistentFlags().String(export.IMPORT_LEVELDB_PATH_CLI, "", "path to import leveldb")
	stateSnapshotCmd.PersistentFlags().String(export.IMPORT_ANCIENT_PATH_CLI, "", "path to import ancient datastore")
	stateSnapshotCmd.PersistentFlags().Uint64(export.SYNC_EXPORT_HEIGHT_CLI, 0, "block height to perform sync export for")
	stateSnapshotCmd.PersistentFlags().Uint64(export.SYNC_EXPORT_SEGMENT_SIZE_CLI, 0, "segment size to chunk block export/import by")
	stateSnapshotCmd.PersistentFlags().Uint(export.SYNC_EXPORT_TRIE_WORKERS_CLI, 0, "number of trie workers for state export")
	stateSnapshotCmd.PersistentFlags().String(export.SYNC_EXPORT_RECOVERY_FILE_CLI, "", "recovery file for state export")

	viper.BindPFlag(export.EXPORT_LEVELDB_PATH_TOML, stateSnapshotCmd.PersistentFlags().Lookup(export.EXPORT_LEVELDB_PATH_CLI))
	viper.BindPFlag(export.EXPORT_ANCIENT_PATH_TOML, stateSnapshotCmd.PersistentFlags().Lookup(export.EXPORT_ANCIENT_PATH_CLI))
	viper.BindPFlag(export.IMPORT_LEVELDB_PATH_TOML, stateSnapshotCmd.PersistentFlags().Lookup(export.IMPORT_LEVELDB_PATH_CLI))
	viper.BindPFlag(export.IMPORT_ANCIENT_PATH_TOML, stateSnapshotCmd.PersistentFlags().Lookup(export.IMPORT_ANCIENT_PATH_CLI))
	viper.BindPFlag(export.SYNC_EXPORT_HEIGHT_TOML, stateSnapshotCmd.PersistentFlags().Lookup(export.SYNC_EXPORT_HEIGHT_CLI))
	viper.BindPFlag(export.SYNC_EXPORT_SEGMENT_SIZE_TOML, stateSnapshotCmd.PersistentFlags().Lookup(export.SYNC_EXPORT_SEGMENT_SIZE_CLI))
	viper.BindPFlag(export.SYNC_EXPORT_TRIE_WORKERS_TOML, stateSnapshotCmd.PersistentFlags().Lookup(export.SYNC_EXPORT_TRIE_WORKERS_CLI))
	viper.BindPFlag(export.SYNC_EXPORT_RECOVERY_FILE_TOML, stateSnapshotCmd.PersistentFlags().Lookup(export.SYNC_EXPORT_RECOVERY_FILE_CLI))
}
