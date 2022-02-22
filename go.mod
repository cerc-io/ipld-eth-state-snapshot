module github.com/vulcanize/eth-pg-ipfs-state-snapshot

go 1.15

require (
	github.com/btcsuite/btcd v0.22.0-beta // indirect
	github.com/cespare/xxhash/v2 v2.1.2 // indirect
	github.com/ethereum/go-ethereum v1.10.15
	github.com/fsnotify/fsnotify v1.5.1 // indirect
	github.com/go-kit/kit v0.10.0 // indirect
	github.com/golang/mock v1.6.0
	github.com/google/go-cmp v0.5.6 // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/gopherjs/gopherjs v0.0.0-20190430165422-3e4dfb77656c // indirect
	github.com/ipfs/go-cid v0.1.0
	github.com/ipfs/go-datastore v0.5.1 // indirect
	github.com/ipfs/go-ipfs-blockstore v1.1.2
	github.com/ipfs/go-ipfs-ds-help v1.1.0
	github.com/ipfs/go-log v1.0.5 // indirect
	github.com/ipfs/go-log/v2 v2.4.0 // indirect
	github.com/jackc/pgx/v4 v4.15.0
	github.com/kr/pretty v0.3.0 // indirect
	github.com/multiformats/go-base32 v0.0.4 // indirect
	github.com/multiformats/go-multihash v0.1.0
	github.com/onsi/ginkgo v1.16.5 // indirect
	github.com/onsi/gomega v1.13.0 // indirect
	github.com/sirupsen/logrus v1.6.0
	github.com/smartystreets/assertions v1.0.0 // indirect
	github.com/spf13/cobra v1.0.0
	github.com/spf13/viper v1.7.0
	github.com/vulcanize/go-eth-state-node-iterator v1.0.1
	go.uber.org/atomic v1.9.0 // indirect
	go.uber.org/goleak v1.1.11 // indirect
	go.uber.org/multierr v1.7.0 // indirect
	go.uber.org/zap v1.19.1 // indirect
	golang.org/x/crypto v0.0.0-20211209193657-4570a0811e8b // indirect
	golang.org/x/lint v0.0.0-20200302205851-738671d3881b // indirect
	golang.org/x/net v0.0.0-20211209124913-491a49abca63 // indirect
	golang.org/x/sys v0.0.0-20211209171907-798191bca915 // indirect
	golang.org/x/tools v0.1.8 // indirect
	google.golang.org/appengine v1.6.6 // indirect
	google.golang.org/protobuf v1.27.1 // indirect
	lukechampine.com/blake3 v1.1.7 // indirect
)

replace github.com/ethereum/go-ethereum v1.10.15 => github.com/vulcanize/go-ethereum v1.10.15-statediff-3.0.1
