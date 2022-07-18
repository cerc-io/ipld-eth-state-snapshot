# ipld-eth-state-snapshot

> Tool for extracting the entire Ethereum state at a particular block height from leveldb into Postgres-backed IPFS

[![Go Report Card](https://goreportcard.com/badge/github.com/vulcanize/ipld-eth-state-snapshot)](https://goreportcard.com/report/github.com/vulcanize/ipld-eth-state-snapshot)

## Setup

* Build the binary:

    ```bash
    make build
    ```

## Configuration

Config format:

```toml
[snapshot]
    mode         = "file"           # indicates output mode <postgres | file>
    workers      = 4                # degree of concurrency, the state trie is subdivided into sections that are traversed and processed concurrently
    blockHeight  = -1               # blockheight to perform the snapshot at (-1 indicates to use the latest blockheight found in leveldb)
    recoveryFile = "recovery_file"  # specifies a file to output recovery information on error or premature closure

[leveldb]
    # path to geth leveldb
    path    = "/Users/user/Library/Ethereum/geth/chaindata"         # ANCIENT_DB_PATH
    # path to geth ancient database
    ancient = "/Users/user/Library/Ethereum/geth/chaindata/ancient" # LVL_DB_PATH

[database]
    # when operating in 'postgres' output mode
    # db credentials
    name     = "vulcanize_public"   # DATABASE_NAME
    hostname = "localhost"          # DATABASE_HOSTNAME
    port     = 5432                 # DATABASE_PORT
    user     = "postgres"           # DATABASE_USER
    password = ""                   # DATABASE_PASSWORD

[file]
    # when operating in 'file' output mode
    # directory the CSV files are written to
    outputDir = "output_dir/"   # FILE_OUTPUT_DIR

[log]
    level = "info"      # log level (trace, debug, info, warn, error, fatal, panic) (default: info)
    file  = "log_file"  # file path for logging, leave unset to log to stdout

[prom]
    # prometheus metrics
    metrics  = true         # enable prometheus metrics         (default: false)
    http     = true         # enable prometheus http service    (default: false)
    httpAddr = "0.0.0.0"    # prometheus http host              (default: 127.0.0.1)
    httpPort = 9101         # prometheus http port              (default: 8086)
    dbStats  = true         # enable prometheus db stats        (default: false)

[ethereum]
    # node info
    clientName   = "Geth"   # ETH_CLIENT_NAME
    nodeID       = "arch1"  # ETH_NODE_ID
    networkID    = "1"      # ETH_NETWORK_ID
    chainID      = "1"      # ETH_CHAIN_ID
    genesisBlock = "0xd4e56740f876aef8c010b86a40d5f56745a118d0906a34e69aec8c0db1cb8fa3" # ETH_GENESIS_BLOCK
```

## Usage

* For state snapshot from LevelDB:

    ```bash
    ./ipld-eth-state-snapshot stateSnapshot --config={path to toml config file}
    ```

* For in-place snapshot in the database:

    ```bash
    ./ipld-eth-state-snapshot inPlaceStateSnapshot --config={path to toml config file}
    ```

## Tests

* Run unit tests:

    ```bash
    # setup db
    docker-compose up -d

    # run tests after db migrations are run
    make dbtest

    # tear down db
    docker-compose down -v --remove-orphans
    ```

## Import output data in file mode into a database

* When `ipld-eth-state-snapshot stateSnapshot` is run in file mode (`database.type`), the output is in form of CSV files.

* Assuming the output files are located in host's `./output_dir` directory, if the DB is running in docker we need to mount the directory containing the files as a volume in the DB service. Eg:

    ```yaml
    # in docker-compose file
    services:
      ipld-eth-db:
        volumes:
          - ./output_dir:/output_dir
    ```

* Combine output from multiple workers:

    ```bash
    # public.blocks
    cat output_dir/**/public.blocks.csv >> output_dir/public.blocks.csv

    # eth.state_cids
    cat output_dir/**/eth.state_cids.csv > output_dir/eth.state_cids.csv

    # eth.storage_cids
    cat output_dir/**/eth.storage_cids.csv > output_dir/eth.storage_cids.csv
    ```

- De-duplicate data:

    ```bash
    # public.blocks
    sort -u output_dir/public.blocks.csv -o output_dir/public.blocks.csv
    ```

* Start `psql` in the DB container to run import commands:

    ```bash
    docker exec -it <CONTAINER_ID> psql -U <DATABASE_USER> <DATABASE_NAME>
    ```

* Run the following to import data:

    ```bash
    # public.nodes
    COPY public.nodes FROM '/output_dir/public.nodes.csv' CSV;

    # public.blocks
    COPY public.blocks FROM '/output_dir/public.blocks.csv' CSV;

    # eth.header_cids
    COPY eth.header_cids FROM '/output_dir/eth.header_cids.csv' CSV;

    # eth.state_cids
    COPY eth.state_cids FROM '/output_dir/eth.state_cids.csv' CSV FORCE NOT NULL state_leaf_key;

    # eth.storage_cids
    COPY eth.storage_cids FROM '/output_dir/eth.storage_cids.csv' CSV FORCE NOT NULL storage_leaf_key;
    ```
