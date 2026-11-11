#!/bin/sh
set -eu

data_root() {
  if [ "$1" != true ]; then return 0; fi
  printf '/mnt/data/%s' "$2"
}

datasource_env() {
  if [ "$1" != true ]; then return 0; fi
  printf 'YON_DATASOURCE_DRIVER=fylo\nYON_DATASOURCE_URL=fylo:///mnt/data/%s\n' "$2"
}

main() {
  app_root="$1" service="$2" fylo="$3"
  printf '{ "server": { "host": "127.0.0.1" } }\n' > "$app_root/server/application-lambda.json"
  datasource_env "$fylo" "$service" > "$app_root/.env"
  cat "$app_root/server/application-lambda.json" "$app_root/.env" >&2
  data_root "$fylo" "$service"
}

main "$@"
