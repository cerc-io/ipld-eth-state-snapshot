# Data correction

* If the state snapshot process running in `file` mode dumps data that can't readily be imported into `ipld-eth-db` because of column count mismatch, following steps can be taken to find and remove the erroneous rows.

* The missing data can be constructed manually from the bad rows in some cases. 

## Find bad data

* For a given table in the `ipld-eth-db` schema, we know the number of columns to be expected in each row in the data dump. 

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

## Delete bad data

* Run the following command to select rows other than the ones having unexpected number of columns:

  ```bash
  ./scripts/delete-bad-rows.sh -i <input-file> -c <expected-columns> -o <output-file>
  ```

  * `input-file` `-i`: Input data file path
  * `expected-columns` `-c`: Expected number of columns in each row of the input file
  * `output-file` `-o`: Output destination file path

    Eg:

    ```bash
    ./scripts/delete-bad-rows.sh -i eth.state_cids.csv -c 8 -o cleaned-eth.state_cids.csv
    ```
