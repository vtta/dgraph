#!/bin/bash
set -ex

DIR="$(dirname "$(readlink -f "$0")")"
DIR="$(readlink -f "$DIR/../..")"
cd "$DIR"

DGRAPH="$DIR/dgraph/dgraph"
[ -f "$DGRAPH" ] || make -j
PIDS=()

unset http_proxy https_proxy
cleanup() {
  kill ${PIDS[@]} &>/dev/null
  wait ${PIDS[@]}
  rm -rf zero alpha*
}
trap cleanup EXIT

wait_for_ctrl_c() {
  read -r -d '' _ </dev/tty
}

zero() {
  NAME=zero
  rm -rf $NAME
  "$DGRAPH" zero -v2 \
    --log_dir=$NAME \
    --wal=$NAME/w \
    --telemetry="reports=false;sentry=false;" \
    &
    # &>/dev/null &
  PIDS+=($!)
  sleep 5
}

alpha() {
  ID=$1
  NAME=alpha$ID
  rm -rf $NAME
  "$DGRAPH" alpha -v2 \
    --security "whitelist=10.0.0.0/8,172.16.0.0/12,192.168.0.0/16;" \
    --log_dir=$NAME \
    --my=0.0.0.0:$((7080+ID)) \
    --zero=0.0.0.0:5080 \
    --port_offset=$ID \
    --tmp=$NAME/t \
    --wal=$NAME/w \
    --postings=$NAME/p \
    --telemetry="reports=false;sentry=false;" \
    &
    # &>/dev/null &
  PIDS+=($!)
}

zero
alpha 0
alpha 1
alpha 2

wait_for_ctrl_c

