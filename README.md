# ipld-eth-state-snapshot

> Tool for extracting the entire Ethereum state at a particular block height from leveldb into Postgres-backed IPFS

[![Go Report Card](https://goreportcard.com/badge/github.com/vulcanize/ipld-eth-state-snapshot)](https://goreportcard.com/report/github.com/vulcanize/ipld-eth-state-snapshot)

## Usage

For state snapshot from LevelDB
```bash
./ipld-eth-state-snapshot stateSnapshot --config={path to toml config file}
```

For in-place snapshot in database
```bash
./ipld-eth-state-snapshot inPlaceStateSnapshot --config={path to toml config file}
```

### Config

Config format:

```toml
[snapshot]
    mode = "file" # indicates output mode ("postgres" or "file")
    workers = 4 # degree of concurrency, the state trie is subdivided into sectiosn that are traversed and processed concurrently
    blockHeight = -1 # blockheight to perform the snapshot at (-1 indicates to use the latest blockheight found in leveldb)
    recoveryFile = "recovery_file" # specifies a file to output recovery information on error or premature closure

[leveldb]
    path = "/Users/user/Library/Ethereum/geth/chaindata" # path to geth leveldb
    ancient = "/Users/user/Library/Ethereum/geth/chaindata/ancient" # path to geth ancient database

[database]
    name     = "vulcanize_public" # postgres database name
    hostname = "localhost" # postgres host
    port     = 5432 # postgres port
    user     = "postgres" # postgres user
    password = "" # postgres password

[file]
    outputDir = "output_dir/" # when operating in 'file' output mode, this is the directory the files are written to

[log]
    level = "info" # log level (trace, debug, info, warn, error, fatal, panic) (default: info)
    file = "log_file" # file path for logging

[prom]
    metrics = true # enable prometheus metrics (default: false)
    http = true # enable prometheus http service (default: false)
    httpAddr = "0.0.0.0" # prometheus http host (default: 127.0.0.1)
    httpPort = 9101 # prometheus http port (default: 8086)
    dbStats = true # enable prometheus db stats (default: false)

# node info
[ethereum]
    clientName = "Geth" # $ETH_CLIENT_NAME
    nodeID = "arch1" # $ETH_NODE_ID
    networkID = "1" # $ETH_NETWORK_ID
    chainID = "1" # $ETH_CHAIN_ID
    genesisBlock = "0xd4e56740f876aef8c010b86a40d5f56745a118d0906a34e69aec8c0db1cb8fa3" # $ETH_GENESIS_BLOCK
```

## Tests

* Install [mockgen](https://github.com/golang/mock#installation)
* `make test`
