#!/bin/bash

# flags
# -i <input-file>:        Input data file
# -c <expected-columns>:  Expected number of columns in each row in the input file
# -o [output-file]:       Output destination file (default: STDOUT)
# -d [true | false]:      Whether to include the data row in output (default: false)

# eg: ./scripts/find-bad-data.sh -i eth.state_cids.csv -c 8 -o res.txt -d true
# output: 1 9 1500000,xxxxxxxx,0x83952d392f9b0059eea94b10d1a095eefb1943ea91595a16c6698757127d4e1c,,
#         baglacgzasvqcntdahkxhufdnkm7a22s2eetj6mx6nzkarwxtkvy4x3bubdgq,\x0f,0,f,/blocks/,
#         DMQJKYBGZRQDVLT2CRWVGPQNNJNCCJU7GL7G4VAI3LZVK4OL5Q2ARTI

while getopts i:c:o:d: OPTION
do
  case "${OPTION}" in
    i) inputFile=${OPTARG};;
    c) expectedColumns=${OPTARG};;
    o) outputFile=${OPTARG};;
    d) data=${OPTARG};;
  esac
done

# if data requested, dump row number, number of columns and the row
if [ "${data}" = true ] ; then
  if [ -z "${outputFile}" ]; then
    awk -F"," "NF!=${expectedColumns} {print NR, NF, \$0}" < ${inputFile}
  else
    awk -F"," "NF!=${expectedColumns} {print NR, NF, \$0}" < ${inputFile} > ${outputFile}
  fi
# else, dump only row number with number of columns
else
  if [ -z "${outputFile}" ]; then
    awk -F"," "NF!=${expectedColumns} {print NR, NF}" < ${inputFile}
  else
    awk -F"," "NF!=${expectedColumns} {print NR, NF}" < ${inputFile} > ${outputFile}
  fi
fi
