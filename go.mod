module github.com/vulcanize/eth-pg-ipfs-state-snapshot

go 1.13

require (
	github.com/dgraph-io/badger v1.6.1 // indirect
	github.com/ethereum/go-ethereum v1.10.9
	github.com/ipfs/go-datastore v0.4.4 // indirect
	github.com/ipfs/go-ipfs-blockstore v1.0.1
	github.com/ipfs/go-ipfs-ds-help v1.0.0
	github.com/libp2p/go-libp2p-kad-dht v0.7.11 // indirect
	github.com/libp2p/go-nat v0.0.5 // indirect
	github.com/multiformats/go-multihash v0.0.14
	github.com/sirupsen/logrus v1.7.0
	github.com/spf13/cobra v1.0.0
	github.com/spf13/viper v1.7.1
	github.com/vulcanize/go-eth-state-node-iterator v0.0.1-alpha
	github.com/vulcanize/ipfs-blockchain-watcher v0.0.11-alpha
)

replace github.com/ethereum/go-ethereum v1.10.9 => github.com/vulcanize/go-ethereum v1.10.9-statediff-0.0.27

replace github.com/vulcanize/go-eth-state-node-iterator => ../state-node-iterator
