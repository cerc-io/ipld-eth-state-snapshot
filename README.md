# ipld-eth-state-snapshot

> Tool for extracting the entire Ethereum state at a particular block height from leveldb into Postgres-backed IPFS

[![Go Report Card](https://goreportcard.com/badge/github.com/vulcanize/ipld-eth-state-snapshot)](https://goreportcard.com/report/github.com/vulcanize/ipld-eth-state-snapshot)

## Usage

./ipld-eth-state-snapshot stateSnapshot --config={path to toml config file}

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
```
