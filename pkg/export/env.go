package export

const (
	EXPORT_LEVELDB_PATH       = "EXPORT_LEVELDB_PATH"
	EXPORT_ANCIENT_PATH       = "EXPORT_ANCIENT_PATH"
	IMPORT_LEVELDB_PATH       = "IMPORT_LEVELDB_PATH"
	IMPORT_ANCIENT_PATH       = "IMPORT_ANCIENT_PATH"
	SYNC_EXPORT_HEIGHT        = "SYNC_EXPORT_HEIGHT"
	SYNC_EXPORT_TRIE_WORKERS  = "SYNC_EXPORT_TRIE_WORKERS"
	SYNC_EXPORT_SEGMENT_SIZE  = "SYNC_EXPORT_SEGMENT_SIZE"
	SYNC_EXPORT_RECOVERY_FILE = "SYNC_EXPORT_RECOVERY_FILE"
)

// TOML bindings
const (
	EXPORT_LEVELDB_PATH_TOML       = "sync.exportLeveldb"
	EXPORT_ANCIENT_PATH_TOML       = "sync.exportAncient"
	IMPORT_LEVELDB_PATH_TOML       = "sync.importLeveldb"
	IMPORT_ANCIENT_PATH_TOML       = "sync.importAncient"
	SYNC_EXPORT_HEIGHT_TOML        = "sync.height"
	SYNC_EXPORT_TRIE_WORKERS_TOML  = "sync.trieWorkers"
	SYNC_EXPORT_SEGMENT_SIZE_TOML  = "sync.segmentSize"
	SYNC_EXPORT_RECOVERY_FILE_TOML = "sync.recoverFile"
)

// CLI flags
const (
	EXPORT_LEVELDB_PATH_CLI       = "sync-export-leveldb"
	EXPORT_ANCIENT_PATH_CLI       = "sync-export-ancient"
	IMPORT_LEVELDB_PATH_CLI       = "sync-import-leveldb"
	IMPORT_ANCIENT_PATH_CLI       = "sync-import-ancient"
	SYNC_EXPORT_HEIGHT_CLI        = "sync-height"
	SYNC_EXPORT_TRIE_WORKERS_CLI  = "sync-trie-workers"
	SYNC_EXPORT_SEGMENT_SIZE_CLI  = "sync-segment-size"
	SYNC_EXPORT_RECOVERY_FILE_CLI = "sync-recovery-file"
)
