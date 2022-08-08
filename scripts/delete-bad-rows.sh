#!/bin/bash

# flags
# -i <input-file>:        Input data file
# -c <expected-columns>:  Expected number of columns in each row in the input file
# -o [output-file]:       Output destination file

# eg: ./scripts/delete-bad-rows.sh -i eth.state_cids.csv -c 8 -o cleaned-eth.state_cids.csv

while getopts i:c:o: OPTION
do
  case "${OPTION}" in
    i) inputFile=${OPTARG};;
    c) expectedColumns=${OPTARG};;
    o) outputFile=${OPTARG};;
  esac
done

timestamp=$(date +%s)

# select only rows having expected number of columns
if [ -z "${outputFile}" ]; then
  echo "Invalid destination file arg (-o) ${outputFile}"
else
  awk -F"," "NF==${expectedColumns}" ${inputFile} > ${outputFile}
fi

difference=$(($(date +%s)-timestamp))
echo Time taken: $(date -d@${difference} -u +%H:%M:%S)
