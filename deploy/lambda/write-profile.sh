#!/bin/sh
set -eu

data_root() {
  if [ "$1" != true ]; then return 0; fi
  printf '/mnt/data/%s' "$2"
}

datasource_json() {
  if [ "$1" != true ]; then return 0; fi
  printf ',\n  "datasource": { "driver": "fylo", "url": "fylo:///mnt/data/%s" }' "$2"
}

main() {
  app_root="$1" service="$2" fylo="$3"
  {
    printf '{\n  "server": { "host": "127.0.0.1" }'
    datasource_json "$fylo" "$service"
    printf '\n}\n'
  } > "$app_root/server/application-lambda.json"
  cat "$app_root/server/application-lambda.json" >&2
  data_root "$fylo" "$service"
}

main "$@"
