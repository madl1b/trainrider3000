#!/bin/sh

infile="stopMappings.csv.csv"

outfile="mappings.out"

awk -F'"' '{split($0,a,","); if(a[3] ~ /Light Rail$/ || a[5] ~ /Light Rail$/) exit; print $1,$3}' $infile | cut -d',' -f3,5 > $outfile