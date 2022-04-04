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

package snapshot

/*
	viper.BindPFlag("snapshot.blockHeight", stateSnapshotCmd.PersistentFlags().Lookup("block-height"))
	viper.BindPFlag("snapshot.workers", stateSnapshotCmd.PersistentFlags().Lookup("workers"))
	viper.BindPFlag("snapshot.recoveryFile", stateSnapshotCmd.PersistentFlags().Lookup("recovery-file"))
	viper.BindPFlag("snapshot.mode", stateSnapshotCmd.PersistentFlags().Lookup("snapshot-mode"))
*/
const (
	SNAPSHOT_BLOCK_HEIGHT  = "SNAPSHOT_BLOCK_HEIGHT"
	SNAPSHOT_WORKERS       = "SNAPSHOT_WORKERS"
	SNAPSHOT_RECOVERY_FILE = "SNAPSHOT_RECOVERY_FILE"
	SNAPSHOT_MODE          = "SNAPSHOT_MODE"

	LOGRUS_LEVEL = "LOGRUS_LEVEL"
	LOGRUS_FILE  = "LOGRUS_FILE"

	FILE_OUTPUT_DIR = "FILE_OUTPUT_DIR"

	ANCIENT_DB_PATH = "ANCIENT_DB_PATH"
	LVL_DB_PATH     = "LVL_DB_PATH"

	ETH_CLIENT_NAME   = "ETH_CLIENT_NAME"
	ETH_GENESIS_BLOCK = "ETH_GENESIS_BLOCK"
	ETH_NETWORK_ID    = "ETH_NETWORK_ID"
	ETH_NODE_ID       = "ETH_NODE_ID"
	ETH_CHAIN_ID      = "ETH_CHAIN_ID"

	DATABASE_NAME                 = "DATABASE_NAME"
	DATABASE_HOSTNAME             = "DATABASE_HOSTNAME"
	DATABASE_PORT                 = "DATABASE_PORT"
	DATABASE_USER                 = "DATABASE_USER"
	DATABASE_PASSWORD             = "DATABASE_PASSWORD"
	DATABASE_MAX_IDLE_CONNECTIONS = "DATABASE_MAX_IDLE_CONNECTIONS"
	DATABASE_MAX_OPEN_CONNECTIONS = "DATABASE_MAX_OPEN_CONNECTIONS"
	DATABASE_MAX_CONN_LIFETIME    = "DATABASE_MAX_CONN_LIFETIME"
)

const (
	SNAPSHOT_BLOCK_HEIGHT_TOML  = "snapshot.blockHeight"
	SNAPSHOT_WORKERS_TOML       = "snapshot.workers"
	SNAPSHOT_RECOVERY_FILE_TOML = "snapshot.recoveryFile"
	SNAPSHOT_MODE_TOML          = "snapshot.mode"

	LOGRUS_LEVEL_TOML = "log.level"
	LOGRUS_FILE_TOML  = "log.file"

	FILE_OUTPUT_DIR_TOML = "file.outputDir"

	ANCIENT_DB_PATH_TOML = "leveldb.ancient"
	LVL_DB_PATH_TOML     = "leveldb.path"

	ETH_CLIENT_NAME_TOML   = "ethereum.clientName"
	ETH_GENESIS_BLOCK_TOML = "ethereum.genesisBlock"
	ETH_NETWORK_ID_TOML    = "ethereum.networkID"
	ETH_NODE_ID_TOML       = "ethereum.nodeID"
	ETH_CHAIN_ID_TOML      = "ethereum.chainID"

	DATABASE_NAME_TOML                 = "database.name"
	DATABASE_HOSTNAME_TOML             = "database.hostname"
	DATABASE_PORT_TOML                 = "database.port"
	DATABASE_USER_TOML                 = "database.user"
	DATABASE_PASSWORD_TOML             = "database.password"
	DATABASE_MAX_IDLE_CONNECTIONS_TOML = "database.maxIdle"
	DATABASE_MAX_OPEN_CONNECTIONS_TOML = "database.maxOpen"
	DATABASE_MAX_CONN_LIFETIME_TOML    = "database.maxLifetime"
)
