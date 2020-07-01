module github.com/vulcanize/eth-pg-ipfs-state-snapshot

go 1.13

require (
	github.com/ethereum/go-ethereum v1.9.11
	github.com/multiformats/go-multihash v0.0.13
	github.com/sirupsen/logrus v1.6.0
	github.com/spf13/cobra v1.0.0
	github.com/spf13/viper v1.7.0
	github.com/vulcanize/ipfs-blockchain-watcher v0.0.11-alpha
)

replace github.com/ethereum/go-ethereum v1.9.11 => github.com/vulcanize/go-ethereum v1.9.11-statediff-0.0.2
