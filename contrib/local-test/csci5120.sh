#!/bin/bash
set -ex

DIR="$(dirname "$(readlink -f "$0")")"
DIR="$(readlink -f "$DIR/../..")"
cd "$DIR"

DGRAPH="$DIR/dgraph/dgraph"
RATEL_RELEASE="https://github.com/dgraph-io/ratel/releases/download/21.03/dgraph-ratel-linux.tar.gz"
RATEL="$DIR/ratel"
[ -f "$DGRAPH" ] || make -j
[ -f "$RATEL" ] || ( wget "$RATEL_RELEASE" && tar axvf dgraph-ratel-linux.tar.gz )
PIDS=()

unset http_proxy https_proxy
cleanup() {
  kill ${PIDS[@]} &>/dev/null
  wait ${PIDS[@]}
  # rm -rf "$DIR/data"
}
trap cleanup EXIT

wait_for_ctrl_c() {
  read -r -d '' _ </dev/tty
}

zero() {
  DATADIR="$DIR/data/zero"
  "$DGRAPH" zero -v2 \
    --log_dir=$DATADIR \
    --wal=$DATADIR/w \
    --telemetry="reports=false;sentry=false;" \
    &
    # &>/dev/null &
  PIDS+=($!)
  sleep 5
}

alpha() {
  ID=$1
  DATADIR="$DIR/data/alpha$ID"
  "$DGRAPH" alpha -v2 \
    --security "whitelist=0.0.0.0/0;" \
    --log_dir=$DATADIR \
    --my=0.0.0.0:$((7080+ID)) \
    --zero=0.0.0.0:5080 \
    --port_offset=$ID \
    --tmp=$DATADIR/t \
    --wal=$DATADIR/w \
    --postings=$DATADIR/p \
    --telemetry="reports=false;sentry=false;" \
    &
    # &>/dev/null &
  PIDS+=($!)
}

ratel() {
  "$RATEL" &
  PIDS+=($!)
}

ratel
zero
alpha 0
alpha 1
alpha 2

wait_for_ctrl_c

