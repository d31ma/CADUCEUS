#!/usr/bin/env bash
set -euo pipefail
# shellcheck source-path=SCRIPTDIR/../floci
. "$(dirname "$0")/../floci/common.sh"

usage() {
  cat << 'TEXT'
Usage: scripts/native/run-all.sh [--up | --down] [app ...]

Starts every Yon app (or the named ones) with `ty start` on this Mac, the Tac
apps as static files, and the Mesail router on one origin, then runs the
blackbox tests in tests/blackbox against it and stops everything.

  --up    start and keep running; prints MESAIL_BASE_URL (stop with --down)
  --down  stop what --up started

Settings (environment):
  MESAIL_OWNER        your slice, for example slice-m3 (lock owner, FYLO roots, logs)
  MESAIL_TEST_RUN     go test -run filter for tests/blackbox
  MESAIL_SKIP_STATIC  1 skips the Tac bundles
  MESAIL_KEYCLOAK_URL a running Keycloak (for example http://127.0.0.1:8180);
                      the router then proxies /idp/ to it
  MESAIL_TOKENS_FILE  a file of `export MESAIL_TOKEN_<NAME>=...` lines to load
FYLO roots: ~/Developer/AtlasFylo/mesail/<owner>/native/<service> (emptied at each start).
Logs: ~/Library/Logs/local/CADUCEUS/<owner>/native-<time>/
TEXT
}

ty_binary() {
  echo "${MESAIL_TY:-$HOME/Developer/Tools/tachyon-26.46.02/ty}"
}

fylo_folder() {
  echo "${MESAIL_FYLO_DIR:-$HOME/Developer/Tools/fylo-26.45.07}"
}

atlas_image() {
  echo "$HOME/Developer/AtlasFylo/mesail"
}

run_folder() {
  echo "${TMPDIR:-/tmp}/mesail-native/$MESAIL_OWNER"
}

data_folder() {
  echo "$(atlas_image)/$MESAIL_OWNER/native"
}

inside_lock() {
  local holder="${MAC_HEAVY_JOB##* pid=}"
  [[ "$holder" =~ ^[0-9]+$ ]] && kill -0 "$holder" 2> /dev/null
}

heavy_job() {
  if inside_lock; then "$@"; return; fi
  MAC_HEAVY_COORDINATOR=tachyon-build "$HOME/.local/bin/mac-heavy" --owner "$MESAIL_OWNER" -- "$@"
}

check_atlas_image() {
  if mount | grep -q " on $(atlas_image) "; then return 0; fi
  echo "The FYLO test image is not attached. Attach it (it is shared; never detach it):"
  echo "  diskutil image attach --mountOptions nobrowse --mountPoint $(atlas_image) /Volumes/ATLAS/Mesail/mesail-fylo-test.sparsebundle"
  return 1
}

free_port() {
  python3 -c 'import socket; s = socket.socket(); s.bind(("127.0.0.1", 0)); print(s.getsockname()[1]); s.close()'
}

yon_apps() {
  if [ "$#" -gt 0 ]; then printf '%s\n' "$@"; return; fi
  find "$(mesail_root)/apps" -mindepth 3 -maxdepth 3 -path '*/server/application.json' | awk -F/ '{print $(NF-2)}' | sort
}

owns_fylo() {
  [ "$(jq -r --arg name "$1" '.services[] | select(.name == $name) | .fylo' "$(mesail_stack_file)")" = true ]
}

copy_app() {
  local app="$1"
  mkdir -p "$(run_folder)/apps/$app"
  rsync -a --delete --exclude .tachyon --exclude dist --exclude .env "$(mesail_root)/apps/$app/" "$(run_folder)/apps/$app/"
}

add_datasource() {
  local app="$1"
  if ! owns_fylo "$app"; then return 0; fi
  mkdir -p "$(data_folder)/$app"
  printf 'YON_DATASOURCE_DRIVER=fylo\nYON_DATASOURCE_URL=fylo://%s\n' "$(cd "$(data_folder)/$app" && pwd -P)" >> "$(run_folder)/apps/$app/.env"
}

write_profile() {
  local app="$1"
  printf '{ "server": { "host": "127.0.0.1" } }\n' > "$(run_folder)/apps/$app/server/application-native.json"
  add_datasource "$app"
}

copy_env_file() {
  local app="$1"
  : > "$(run_folder)/apps/$app/.env"
  chmod 600 "$(run_folder)/apps/$app/.env"
  if [ ! -f "$(mesail_root)/apps/$app/.env" ]; then return 0; fi
  grep -v -E '^YON_DATASOURCE_(DRIVER|URL)=' "$(mesail_root)/apps/$app/.env" >> "$(run_folder)/apps/$app/.env" || true
}

prepare_app() {
  local app="$1"
  copy_app "$app"
  copy_env_file "$app"
  write_profile "$app"
  "$(ty_binary)" bundle "$(run_folder)/apps/$app"
}

prepare_apps() {
  local app failed=0
  for app in "$@"; do
    prepare_app "$app" > "$(mesail_log_dir)/prepare-$app.log" 2>&1 &
  done
  wait || failed=1
  for app in "$@"; do
    tail -3 "$(mesail_log_dir)/prepare-$app.log" | sed "s/^/$app: /"
  done
  return "$failed"
}

start_app() {
  local app="$1" port
  port="$(free_port)"
  (cd "$(run_folder)/apps/$app" && PATH="$(fylo_folder):$PATH" YON_PROFILE=native \
    exec "$(ty_binary)" start "$(run_folder)/apps/$app" --port "$port") > "$(mesail_log_dir)/app-$app.log" 2>&1 &
  echo "$!" >> "$(run_folder)/pids"
  echo "$app $!" >> "$(run_folder)/servers"
  echo "$app=http://127.0.0.1:$port" >> "$(run_folder)/upstreams"
  echo "$app: ty start on port $port (pid $!, log app-$app.log)"
}

bundle_static_app() {
  local app="$1" prefix="$2"
  copy_app "$app"
  if "$(ty_binary)" bundle "$(run_folder)/apps/$app" --target web; then
    echo "$prefix=$(run_folder)/apps/$app/dist/web" >> "$(run_folder)/static"
    return 0
  fi
  echo "warning: $app did not bundle; its pages are not served"
}

bundle_static_apps() {
  local app prefix
  : > "$(run_folder)/static"
  if [ "${MESAIL_SKIP_STATIC:-0}" = 1 ]; then echo "Tac bundles skipped (MESAIL_SKIP_STATIC=1)"; return; fi
  while read -r app prefix; do
    mesail_step "bundle-$app" bundle_static_app "$app" "$prefix"
  done < <(mesail_stack '.staticApps[] | "\(.app) \(.prefix)"')
}

joined() {
  paste -sd, "$1"
}

start_router() {
  local port
  port="$(free_port)"
  (cd "$(mesail_root)/deploy/router" && GOFLAGS=-buildvcs=false go build -o "$(run_folder)/mesail-router" .)
  MESAIL_ROUTER_LISTEN="127.0.0.1:$port" MESAIL_ROUTER_UPSTREAMS="$(joined "$(run_folder)/upstreams")" \
    MESAIL_ROUTER_STATIC_FOLDERS="$(joined "$(run_folder)/static")" MESAIL_ROUTER_KEYCLOAK_URL="${MESAIL_KEYCLOAK_URL:-}" \
    "$(run_folder)/mesail-router" > "$(mesail_log_dir)/router.log" 2>&1 &
  echo "$!" >> "$(run_folder)/pids"
  echo "export MESAIL_BASE_URL=http://127.0.0.1:$port" > "$(run_folder)/env"
  echo "router on http://127.0.0.1:$port (pid $!)"
}

service_answers() {
  curl --fail --silent "$1/api/v1/$2/health" > /dev/null
}

wait_for_app() {
  local base="$1" app="$2" pid="$3" waited=0
  until service_answers "$base" "$app"; do
    if ! kill -0 "$pid" 2> /dev/null; then echo "$app stopped:"; tail -20 "$(mesail_log_dir)/app-$app.log"; return 1; fi
    if [ "$waited" -ge "$(start_seconds)" ]; then echo "timed out after $waited s waiting for $app"; return 1; fi
    sleep 1
    waited=$((waited + 1))
  done
  echo "$app answers after ${waited} s"
}

start_seconds() {
  echo "${MESAIL_START_SECONDS:-180}"
}

router_answers() {
  curl --fail --silent "$1/_router/ready" > /dev/null
}

wait_for_services() {
  local base app
  base="$(sed -n 's/^export MESAIL_BASE_URL=//p' "$(run_folder)/env")"
  mesail_wait_for router 30 router_answers "$base"
  while read -r app pid; do
    wait_for_app "$base" "$app" "$pid" || echo "warning: $app did not start; its tests fail"
  done < "$(run_folder)/servers"
}

reset_data() {
  rm -rf "$(data_folder)"
  mkdir -p "$(data_folder)"
}

start_all() {
  local app
  check_atlas_image
  stop_all
  rm -rf "$(run_folder)"
  mkdir -p "$(run_folder)"
  : > "$(run_folder)/pids"
  : > "$(run_folder)/upstreams"
  : > "$(run_folder)/servers"
  reset_data
  # shellcheck disable=SC2046
  mesail_step prepare-apps prepare_apps $(yon_apps "$@")
  for app in $(yon_apps "$@"); do
    mesail_step "start-$app" start_app "$app"
  done
  bundle_static_apps
  mesail_step start-router start_router
  mesail_step wait-services wait_for_services
  cat "$(run_folder)/env"
}

stop_all() {
  local pid
  if [ ! -f "$(run_folder)/pids" ]; then return 0; fi
  while read -r pid; do
    kill "$pid" 2> /dev/null || true
  done < "$(run_folder)/pids"
  rm -f "$(run_folder)/pids"
  echo "stopped the native Mesail of $MESAIL_OWNER"
}

load_tokens() {
  if [ -z "${MESAIL_TOKENS_FILE:-}" ]; then echo "no MESAIL_TOKENS_FILE: tests that need a token skip"; return 0; fi
  # shellcheck disable=SC1090
  . "$MESAIL_TOKENS_FILE"
}

run_blackbox() {
  local filter=()
  if [ -n "${MESAIL_TEST_RUN:-}" ]; then filter=(-run "$MESAIL_TEST_RUN"); fi
  # shellcheck disable=SC1091
  . "$(run_folder)/env"
  (cd "$(mesail_root)/tests/blackbox" && GOFLAGS=-buildvcs=false \
    go test -tags blackbox -count=1 -v ${filter[@]+"${filter[@]}"} ./...)
}

start_test_stop() {
  trap stop_all EXIT
  start_all "$@"
  load_tokens
  mesail_step blackbox run_blackbox
}

main() {
  export MESAIL_OWNER="${MESAIL_OWNER:-native}"
  export MESAIL_LOG_DIR="${MESAIL_LOG_DIR:-$HOME/Library/Logs/local/CADUCEUS/$MESAIL_OWNER/native-$(date +%Y%m%d-%H%M%S)}"
  mkdir -p "$MESAIL_LOG_DIR"
  mesail_load_settings
  case "${1:-}" in
    -h|--help) usage ;;
    --down) stop_all ;;
    --up) shift; echo "native: logs in $(mesail_log_dir)"; heavy_job "$0" --start-only "$@" ;;
    --start-only) shift; start_all "$@" ;;
    --test-only) shift; start_test_stop "$@" ;;
    *) echo "native: logs in $(mesail_log_dir)"; heavy_job "$0" --test-only "$@" ;;
  esac
}

main "$@"
