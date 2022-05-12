module github.com/vulcanize/ipld-eth-state-snapshot

go 1.15

require (
	github.com/btcsuite/btcd/btcec/v2 v2.2.0 // indirect
	github.com/ethereum/go-ethereum v1.10.17
	github.com/golang/mock v1.6.0
	github.com/ipfs/go-cid v0.1.0
	github.com/ipfs/go-ipfs-blockstore v1.1.2
	github.com/ipfs/go-ipfs-ds-help v1.1.0
	github.com/jackc/pgx/v4 v4.15.0
	github.com/multiformats/go-multihash v0.1.0
	github.com/sirupsen/logrus v1.6.0
	github.com/spf13/cobra v1.0.0
	github.com/spf13/viper v1.7.0
	github.com/vulcanize/go-eth-state-node-iterator v1.0.1
)

replace github.com/ethereum/go-ethereum v1.10.17 => github.com/vulcanize/go-ethereum v1.10.17-statediff-3.2.0.0.20220512091306-cef1fc425fe4
