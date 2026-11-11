#!/bin/sh
set -eu

data_root() {
  cat /opt/mesail/data-root
}

running_as_root() {
  test "$(id -u)" = 0
}

prepare_data_root() {
  if [ -z "$MESAIL_DATA_ROOT" ]; then return 0; fi
  mkdir -p "$MESAIL_DATA_ROOT"
}

hand_data_root_to_service_user() {
  if [ -z "$MESAIL_DATA_ROOT" ]; then return 0; fi
  chown "$MESAIL_USER_ID:$MESAIL_USER_ID" "$MESAIL_DATA_ROOT"
}

copy_stored() {
  rm -rf "$2"
  mkdir -p "$2"
  cp -R "$1/." "$2/"
}

start_server() {
  copy_stored "$MESAIL_STORED_CACHE" "$TAC_CACHE_DIR"
  copy_stored "$MESAIL_STORED_APP" "$MESAIL_APP_ROOT"
  echo "mesail: starting $MESAIL_SERVICE (data root: ${MESAIL_DATA_ROOT:-none})"
  exec ty start "$MESAIL_APP_ROOT" --port "$AWS_LWA_PORT"
}

start_as_service_user() {
  hand_data_root_to_service_user
  exec setpriv --reuid="$MESAIL_USER_ID" --regid="$MESAIL_USER_ID" --clear-groups "$0" serve
}

main() {
  MESAIL_DATA_ROOT="$(data_root)"
  prepare_data_root
  if [ "${1:-}" = serve ]; then start_server; fi
  if running_as_root; then start_as_service_user; fi
  start_server
}

main "$@"
