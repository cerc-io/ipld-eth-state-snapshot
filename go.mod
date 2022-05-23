module github.com/vulcanize/ipld-eth-state-snapshot

go 1.15

require (
	github.com/btcsuite/btcd/btcec/v2 v2.2.0 // indirect
	github.com/cespare/xxhash/v2 v2.1.2 // indirect
	github.com/ethereum/go-ethereum v1.10.17
	github.com/fsnotify/fsnotify v1.5.1 // indirect
	github.com/go-kit/kit v0.10.0 // indirect
	github.com/golang/mock v1.6.0
	github.com/ipfs/go-cid v0.1.0
	github.com/ipfs/go-ipfs-blockstore v1.1.2
	github.com/ipfs/go-ipfs-ds-help v1.1.0
	github.com/jackc/pgx/v4 v4.15.0
	github.com/multiformats/go-multihash v0.1.0
	github.com/onsi/ginkgo v1.16.5 // indirect
	github.com/onsi/gomega v1.13.0 // indirect
	github.com/prometheus/client_golang v1.3.0
	github.com/sirupsen/logrus v1.6.0
	github.com/spf13/cobra v1.0.0
	github.com/spf13/viper v1.7.0
	github.com/vulcanize/go-eth-state-node-iterator v1.0.2
)

replace github.com/ethereum/go-ethereum v1.10.17 => github.com/vulcanize/go-ethereum v1.10.17-statediff-4.0.1-alpha
