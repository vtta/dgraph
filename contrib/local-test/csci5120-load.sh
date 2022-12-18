#!/bin/bash
set -ex

DIR="$(dirname "$(readlink -f "$0")")"
DIR="$(readlink -f "$DIR/../..")"
cd "$DIR"

DGRAPH="$DIR/dgraph/dgraph"
[ -f "$DGRAPH" ] || make -j
unset http_proxy https_proxy

fail() {
    printf '%s\n' "$1" >&2
    exit "${2-1}"
}

is_dgrahp_up() {
  curl "localhost:8080/query" --silent --request POST \
    --header "Content-Type: application/dql" \
    --data $'{ me(func: has(starring)) { name } }'
}

is_dgrahp_up || \
  fail "please launch dgraph first: bash contrib/local-test/csci5120.sh"
[ $# -eq 1 ] || \
  fail "please provide path to dbpedia dataset in rdf format and schema, e.g. dbpedia/2016-10/core-i18n"

DATASRCDIR="$(readlink -f "$1")"
BULK="$DATASRCDIR/../bulk"
mkdir -p "$BULK/data"

fd -Ie txt part- "$DATASRCDIR/schema.indexed.dgraph" -X cat {} > "$BULK/dgraph.schema"
fd -Ie txt.gz part- "$DATASRCDIR" -x ln -sf {} "$BULK/data"
FILES="$(cd "$BULK/data"; fd -Ie txt.gz part- | xargs | sed -e "s/ /,/g" )"

cd "$BULK/data"
"$DGRAPH" bulk -j=$(nproc) --ignore_errors \
  --zero=localhost:5080 \
  --store_xids --xidmap="$BULK/xidmap" \
  --tmp="$BULK/tmp" --out="$BULK/out" --replace_out \
  --schema="$BULK/dgraph.schema" \
  --format=rdf --files="$FILES" \
  2>&1 | tee "$BULK/log"

# adapted from https://github.com/G-Research/dgraph-dbpedia
#                data                             bulk                        schema
# ./dgraph.bulk.sh $(pwd)/dbpedia/2016-10/core-i18n $(pwd)/dbpedia/2016-10/bulk "/data/schema.indexed.dgraph/dataset=*/lang=*/part-*.txt" "/data/*.rdf/lang=*/part-*.txt.gz"
# cat $DATASRCDIR/schema.indexed.dgraph/dataset=*/lang=*/part-*.txt > "$BULK/dgraph.schema" 
# RDF="$(ls $DATASRCDIR/*.rdf/lang=*/part-*.txt.gz)"

