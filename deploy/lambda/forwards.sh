#!/bin/sh
set -eu

start_forward() {
  local_port="${1%%=*}" target="${1#*=}"
  socat "TCP-LISTEN:$local_port,bind=127.0.0.1,fork,reuseaddr" "TCP:$target" &
  echo "mesail: forwarding 127.0.0.1:$local_port to $target"
}

main() {
  for forward in $(echo "${MESAIL_FORWARDS:-}" | tr ',' ' '); do
    start_forward "$forward"
  done
}

main
