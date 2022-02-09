package types

var TableIPLDBlock = Table{
	`public.blocks`,
	[]column{
		column{"key", text},
		column{"data", bytea},
	},
	`ON CONFLICT (key) DO NOTHING`,
}

var TableNodeInfo = Table{
	Name: `public.nodes`,
	Columns: []column{
		column{"genesis_block", varchar},
		column{"network_id", varchar},
		column{"node_id", varchar},
		column{"client_name", varchar},
		column{"chain_id", integer},
	},
}

var TableHeader = Table{
	"eth.header_cids",
	[]column{
		column{"block_number", bigint},
		column{"block_hash", varchar},
		column{"parent_hash", varchar},
		column{"cid", text},
		column{"td", numeric},
		column{"node_id", varchar},
		column{"reward", numeric},
		column{"state_root", varchar},
		column{"tx_root", varchar},
		column{"receipt_root", varchar},
		column{"uncle_root", varchar},
		column{"bloom", bytea},
		column{"timestamp", numeric},
		column{"mh_key", text},
		column{"times_validated", integer},
		column{"coinbase", varchar},
	},
	"ON CONFLICT (block_hash) DO UPDATE SET (parent_hash, cid, td, node_id, reward, state_root, tx_root, receipt_root, uncle_root, bloom, timestamp, mh_key, times_validated, coinbase) = (EXCLUDED.parent_hash, EXCLUDED.cid, EXCLUDED.td, EXCLUDED.node_id, EXCLUDED.reward, EXCLUDED.state_root, EXCLUDED.tx_root, EXCLUDED.receipt_root, EXCLUDED.uncle_root, EXCLUDED.bloom, EXCLUDED.timestamp, EXCLUDED.mh_key, eth.header_cids.times_validated + 1, EXCLUDED.coinbase)",
}

var TableStateNode = Table{
	"eth.state_cids",
	[]column{
		column{"header_id", varchar},
		column{"state_leaf_key", varchar},
		column{"cid", text},
		column{"state_path", bytea},
		column{"node_type", integer},
		column{"diff", boolean},
		column{"mh_key", text},
	},
	`ON CONFLICT (header_id, state_path) DO UPDATE SET (state_leaf_key, cid, node_type, diff, mh_key) = (EXCLUDED.state_leaf_key, EXCLUDED.cid, EXCLUDED.node_type, EXCLUDED.diff, EXCLUDED.mh_key)`,
}

var TableStorageNode = Table{
	"eth.storage_cids",
	[]column{
		column{"header_id", varchar},
		column{"state_path", bytea},
		column{"storage_leaf_key", varchar},
		column{"cid", text},
		column{"storage_path", bytea},
		column{"node_type", integer},
		column{"diff", boolean},
		column{"mh_key", text},
	},
	"ON CONFLICT (header_id, state_path, storage_path) DO UPDATE SET (storage_leaf_key, cid, node_type, diff, mh_key) = (EXCLUDED.storage_leaf_key, EXCLUDED.cid, EXCLUDED.node_type, EXCLUDED.diff, EXCLUDED.mh_key)",
}
