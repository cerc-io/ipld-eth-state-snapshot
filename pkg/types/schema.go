package types

var TableIPLDBlock = Table{
	`public.blocks`,
	[]column{
		{"block_number", bigint},
		{"key", text},
		{"data", bytea},
	},
	`ON CONFLICT (key, block_number) DO NOTHING`,
}

var TableNodeInfo = Table{
	Name: `public.nodes`,
	Columns: []column{
		{"genesis_block", varchar},
		{"network_id", varchar},
		{"node_id", varchar},
		{"client_name", varchar},
		{"chain_id", integer},
	},
}

var TableHeader = Table{
	"eth.header_cids",
	[]column{
		{"block_number", bigint},
		{"block_hash", varchar},
		{"parent_hash", varchar},
		{"cid", text},
		{"td", numeric},
		{"node_id", varchar},
		{"reward", numeric},
		{"state_root", varchar},
		{"tx_root", varchar},
		{"receipt_root", varchar},
		{"uncle_root", varchar},
		{"bloom", bytea},
		{"timestamp", numeric},
		{"mh_key", text},
		{"times_validated", integer},
		{"coinbase", varchar},
	},
	"ON CONFLICT (block_hash, block_number) DO UPDATE SET (parent_hash, cid, td, node_id, reward, state_root, tx_root, receipt_root, uncle_root, bloom, timestamp, mh_key, times_validated, coinbase) = (EXCLUDED.parent_hash, EXCLUDED.cid, EXCLUDED.td, EXCLUDED.node_id, EXCLUDED.reward, EXCLUDED.state_root, EXCLUDED.tx_root, EXCLUDED.receipt_root, EXCLUDED.uncle_root, EXCLUDED.bloom, EXCLUDED.timestamp, EXCLUDED.mh_key, eth.header_cids.times_validated + 1, EXCLUDED.coinbase)",
}

var TableStateNode = Table{
	"eth.state_cids",
	[]column{
		{"block_number", bigint},
		{"header_id", varchar},
		{"state_leaf_key", varchar},
		{"cid", text},
		{"state_path", bytea},
		{"node_type", integer},
		{"diff", boolean},
		{"mh_key", text},
	},
	`ON CONFLICT (header_id, state_path, block_number) DO UPDATE SET (state_leaf_key, cid, node_type, diff, mh_key) = (EXCLUDED.state_leaf_key, EXCLUDED.cid, EXCLUDED.node_type, EXCLUDED.diff, EXCLUDED.mh_key)`,
}

var TableStorageNode = Table{
	"eth.storage_cids",
	[]column{
		{"block_number", bigint},
		{"header_id", varchar},
		{"state_path", bytea},
		{"storage_leaf_key", varchar},
		{"cid", text},
		{"storage_path", bytea},
		{"node_type", integer},
		{"diff", boolean},
		{"mh_key", text},
	},
	"ON CONFLICT (header_id, state_path, storage_path, block_number) DO UPDATE SET (storage_leaf_key, cid, node_type, diff, mh_key) = (EXCLUDED.storage_leaf_key, EXCLUDED.cid, EXCLUDED.node_type, EXCLUDED.diff, EXCLUDED.mh_key)",
}
