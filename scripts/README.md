## Data Validation

* For a given table in the `ipld-eth-db` schema, we know the number of columns to be expected in each row in the data dump:

  | Table              | Expected columns |
  |--------------------|:----------------:|
  | `public.nodes`     | 5                |
  | `ipld.blocks`      | 3                |
  | `eth.header_cids`  | 16               |
  | `eth.state_cids`   | 8                |
  | `eth.storage_cids` | 9                |

### Find Bad Data

* Run the following command to find any rows having unexpected number of columns:

  ```bash
  ./scripts/find-bad-rows.sh -i <input-file> -c <expected-columns> -o [output-file] -d [include-data]
  ```

  * `input-file` `-i`: Input data file path
  * `expected-columns` `-c`: Expected number of columns in each row of the input file
  * `output-file` `-o`: Output destination file path (default: `STDOUT`)
  * `include-data` `-d`: Whether to include the data row in the output (`true | false`) (default: `false`)
  * The output is of format: row number, number of columns, the data row

    Eg:

    ```bash
    ./scripts/find-bad-rows.sh -i eth.state_cids.csv -c 8 -o res.txt -d true
    ```

    Output:

    ```
    1 9 1500000,xxxxxxxx,0x83952d392f9b0059eea94b10d1a095eefb1943ea91595a16c6698757127d4e1c,,baglacgzasvqcntdahkxhufdnkm7a22s2eetj6mx6nzkarwxtkvy4x3bubdgq,\x0f,0,f,/blocks/,DMQJKYBGZRQDVLT2CRWVGPQNNJNCCJU7GL7G4VAI3LZVK4OL5Q2ARTI
    ```

    Eg:

    ```bash
    ./scripts/find-bad-rows.sh -i public.nodes.csv -c 5 -o res.txt -d true
    ./scripts/find-bad-rows.sh -i ipld.blocks.csv -c 3 -o res.txt -d true
    ./scripts/find-bad-rows.sh -i eth.header_cids.csv -c 16 -o res.txt -d true
    ./scripts/find-bad-rows.sh -i eth.state_cids.csv -c 8 -o res.txt -d true
    ./scripts/find-bad-rows.sh -i eth.storage_cids.csv -c 9 -o res.txt -d true
    ```

## Data Cleanup

* In case of column count mismatch, data from `file` mode dumps can't be imported readily into `ipld-eth-db`.

### Filter Bad Data

* Run the following command to filter out rows having unexpected number of columns:

  ```bash
  ./scripts/filter-bad-rows.sh -i <input-file> -c <expected-columns> -o <output-file>
  ```

  * `input-file` `-i`: Input data file path
  * `expected-columns` `-c`: Expected number of columns in each row of the input file
  * `output-file` `-o`: Output destination file path

    Eg:

    ```bash
    ./scripts/filter-bad-rows.sh -i public.nodes.csv -c 5 -o cleaned-public.nodes.csv
    ./scripts/filter-bad-rows.sh -i ipld.blocks.csv -c 3 -o cleaned-ipld.blocks.csv
    ./scripts/filter-bad-rows.sh -i eth.header_cids.csv -c 16 -o cleaned-eth.header_cids.csv
    ./scripts/filter-bad-rows.sh -i eth.state_cids.csv -c 8 -o cleaned-eth.state_cids.csv
    ./scripts/filter-bad-rows.sh -i eth.storage_cids.csv -c 9 -o cleaned-eth.storage_cids.csv
    ```
