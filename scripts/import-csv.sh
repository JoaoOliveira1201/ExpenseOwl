#!/bin/sh
set -eu

if [ "$#" -lt 1 ] || [ "$#" -gt 2 ]; then
    echo "Usage: $0 PATH_TO_CSV [EXPENSEOWL_URL]" >&2
    exit 2
fi

csv_file=$1
base_url=${2:-http://localhost:${APP_PORT:-8080}}

if [ ! -f "$csv_file" ]; then
    echo "CSV file not found: $csv_file" >&2
    exit 2
fi

curl --fail-with-body --silent --show-error \
    --form "file=@${csv_file};type=text/csv" \
    "${base_url%/}/import/csv"
echo
