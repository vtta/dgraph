#!/bin/bash
set -ex

DIR="$(dirname "$(readlink -f "$0")")"
DIR="$(readlink -f "$DIR/../..")"

DGRAPH="$DIR/dgraph/dgraph"
[ -f "$DGRAPH" ] || make -C "$DIR" -j
unset http_proxy https_proxy
cleanup() {
  kill ${PIDS[@]} &>/dev/null
  wait ${PIDS[@]}
  rm -rf "$TEMP"
}
trap cleanup EXIT

fail() {
    printf '%s\n' "$1" >&2
    exit "${2-1}"
}

zero() {
  local DATADIR="$DIR/data/zero"
  rm -rf $DATADIR
  "$DGRAPH" zero -v2 \
    --log_dir=$DATADIR \
    --wal=$DATADIR/w \
    --telemetry="reports=false;sentry=false;" \
    &
    # &>/dev/null &
  PIDS+=($!)
  sleep 5
}

bulk() {
  [ $# -eq 4 ] || fail "wrong arguement format"
  # files are all relative path under working dir
  local WORKINGDIR="$1"
  local FILES="$2"
  local SCHEMA="$3"
  local SHARDS="$4"
  local DATADIR="$DIR/data/bulk"
  pushd "$WORKINGDIR"
  "$DGRAPH" bulk -j=$((3*$(nproc))) --ignore_errors \
    --reduce_shards="$SHARDS" --map_shards="$((16*SHARDS))" \
    --badger=compression=zstd \
    --zero=localhost:5080 \
    --tmp="$DATADIR/tmp" \
    --out="$DATADIR/out" --replace_out \
    --store_xids --xidmap="$DATADIR/xidmap" \
    --schema="$SCHEMA" \
    --format=rdf --files="$FILES" \
    --log_dir=$DATADIR
  popd
}

[ $# -eq 1 ] || \
  fail "please provide path to dbpedia dataset in rdf format, e.g. dbpedia/2016-10/core-i18n"
DBPEDIA="$(readlink -f "$1")"
TEMP="$(mktemp --directory -t dgraph-bulk-XXXXXXXXXX)"

fd -Ie txt part- "$DBPEDIA/schema.indexed.dgraph" -X cat {} > "$TEMP/schema"
fd -Ie txt.gz part- "$DBPEDIA" -x ln -sf {} "$TEMP"
FILES="$(fd -Ie txt.gz part- "$TEMP" -X echo {/} | xargs | sed -e "s/ /,/g")"

zero
bulk "$TEMP" "$FILES" "schema" 3

