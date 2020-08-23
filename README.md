# eth-pg-ipfs-state-snapshot

> Tool for extracting the entire Ethereum state at a particular block height from leveldb into Postgres-backed IPFS

[![Go Report Card](https://goreportcard.com/badge/github.com/vulcanize/eth-pg-ipfs-state-snapshot)](https://goreportcard.com/report/github.com/vulcanize/eth-pg-ipfs-state-snapshot)

## Usage 

./eth-pg-ipfs-state-snapshot stateSnapshot --config={path to toml config file}

Config format:

```toml
[database]
    name     = "vulcanize_public"
    hostname = "localhost"
    port     = 5432
    user     = "postgres"

[leveldb]
    path = "/Users/user/Library/Ethereum/geth/chaindata"
    # path for geth's "freezer" archive
    ancient = "/Users/user/Library/Ethereum/geth/chaindata"

[snapshot]
    blockHeight = 0
    divideDepth = 1
```
