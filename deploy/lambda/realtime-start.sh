#!/bin/sh
set -eu

yon_port=8106
management_address=127.0.0.1:8117
stopping=0
yon_pid=
gateway_pid=

require_secret() {
  if [ -n "${MESAIL_REALTIME_GATEWAY_SECRET:-}" ]; then return 0; fi
  echo "mesail: MESAIL_REALTIME_GATEWAY_SECRET is not set" >&2
  exit 1
}

reserved_names() {
  printf 'PROFILE|EGRESS_ENFORCEMENT|ISOLATION|PORT|HOST|DATASOURCE_DRIVER|DATASOURCE_URL|REALTIME_GATEWAY_URL|REALTIME_GATEWAY_SECRET'
}

service_secrets() {
  env | grep -E '^YON_[A-Z0-9_]+=' | grep -v -E "^YON_($(reserved_names))=" || true
}

write_env() {
  printf 'YON_DATASOURCE_DRIVER=fylo\n'
  printf 'YON_DATASOURCE_URL=fylo://%s\n' "$MESAIL_DATA_ROOT"
  printf 'YON_REALTIME_GATEWAY_URL=http://%s\n' "$management_address"
  printf 'YON_REALTIME_GATEWAY_SECRET=%s\n' "$MESAIL_REALTIME_GATEWAY_SECRET"
  service_secrets
}

prepare_app() {
  mkdir -p "$MESAIL_DATA_ROOT"
  rm -f "$MESAIL_APP_ROOT/.env"
  (umask 077 && write_env > "$MESAIL_APP_ROOT/.env")
}

start_yon() {
  ty start "$MESAIL_APP_ROOT" --host 0.0.0.0 --port "$yon_port" --allow-non-loopback &
  yon_pid=$!
}

start_gateway() {
  MESAIL_REALTIME_MANAGE_LISTEN="$management_address" MESAIL_REALTIME_BACKEND="http://127.0.0.1:$yon_port" \
    mesail-realtime-gateway &
  gateway_pid=$!
}

signal_children() {
  kill -TERM "$yon_pid" "$gateway_pid" 2> /dev/null || true
}

keep_watching() {
  [ "$stopping" = 0 ] && kill -0 "$yon_pid" 2> /dev/null && kill -0 "$gateway_pid" 2> /dev/null
}

watch_children() {
  while keep_watching; do sleep 1; done
}

finish() {
  if [ "$stopping" = 0 ]; then echo "mesail: a realtime process ended, so the container stops" >&2; fi
  signal_children
  wait || true
  if [ "$stopping" = 0 ]; then exit 1; fi
  exit 0
}

main() {
  require_secret
  prepare_app
  trap 'stopping=1; signal_children' TERM INT
  echo "mesail: starting realtime (data root: $MESAIL_DATA_ROOT, Yon on port $yon_port, gateway on $MESAIL_REALTIME_LISTEN)"
  /usr/local/lib/mesail/forwards.sh
  start_yon
  start_gateway
  watch_children
  finish
}

main "$@"
