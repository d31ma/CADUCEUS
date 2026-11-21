#!/usr/bin/env bash
set -euo pipefail

banner() {
  local host="$1" port="$2" line
  exec 3<> "/dev/tcp/$host/$port" || return 1
  IFS= read -r -t 5 line <&3
  printf 'QUIT\r\n' >&3
  exec 3>&-
  echo "$line"
}

main() {
  local address="${1:?usage: smtp-ready.sh <host:port>}"
  [[ "$(banner "${address%:*}" "${address##*:}")" == 220* ]]
}

main "$@"
