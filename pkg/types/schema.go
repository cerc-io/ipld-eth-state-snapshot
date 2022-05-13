package types

var TableIPLDBlock = Table{
	`public.blocks`,
	[]column{
		{"block_number", bigint},
		{"key", text},
		{"data", bytea},
	},
	`ON CONFLICT DO NOTHING`,
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
	"ON CONFLICT DO NOTHING",
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
	`ON CONFLICT DO NOTHING`,
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
	"ON CONFLICT DO NOTHING",
}
