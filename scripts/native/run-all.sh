#!/usr/bin/env bash
set -euo pipefail
# shellcheck source-path=SCRIPTDIR/../floci
. "$(dirname "$0")/../floci/common.sh"
# shellcheck source-path=SCRIPTDIR
. "$(dirname "$0")/keycloak.sh"

usage() {
  cat << 'TEXT'
Usage: scripts/native/run-all.sh [--up | --down] [app ...]

Starts every Yon app (or the named ones) with `ty start` on this Mac, the Tac
apps as static files, and the Mesail router on one origin, then runs the
blackbox tests in tests/blackbox against it and stops everything.

  --up      start and keep running; prints MESAIL_BASE_URL (stop with --down)
  --tokens  write fresh tokens for the running --up stack (they last 60 s)
  --down    stop what --up started

Settings (environment):
  MESAIL_OWNER        your slice, for example slice-m3 (lock owner, FYLO roots, logs)
  MESAIL_TEST_RUN     go test -run filter for tests/blackbox
  MESAIL_SKIP_STATIC  1 skips the Tac bundles
  MESAIL_KEYCLOAK      native (default): start Keycloak 26.8.2 natively with the realms
                      example.com, example.org, partner.test, mesail-operators and the
                      blackbox users (ana, bo, eve, carol, operator), then write the tokens
                      file and add example.com and example.org to the directory; none: skip
  MESAIL_ROUTER_PASSTHROUGH  extra routes for the router, for example the realtime gateway:
                      /api/v1/realtime/socket=http://127.0.0.1:8116,/api/v1/realtime/stream=http://127.0.0.1:8116
  MESAIL_TOKENS_FILE  where the tokens go (default ~/Developer/AtlasFylo/mesail/<owner>/tokens.env,
                      mode 600); with MESAIL_KEYCLOAK=none, a file you made to load instead
  MESAIL_SECRETS_FILE NAME=value client secrets (as in .env.example); generated per run when unset.
                      Each app gets the ones it uses unless its own .env already sets them.
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
  MAC_HEAVY_COORDINATOR=caduceus-build "$HOME/.local/bin/mac-heavy" --owner "$MESAIL_OWNER" -- "$@"
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

app_root_of() {
  mkdir -p "$(data_folder)/$1"
  cd "$(data_folder)/$1" && pwd -P
}

declares_datasource() {
  grep -q '"datasource"' "$(run_folder)/apps/$1/server/application.json"
}

point_copy_at_this_run() {
  local app="$1" file
  file="$(run_folder)/apps/$app/server/application.json"
  sed -E -i '' -e "s#fylo:///mnt/data/$app#fylo://$(app_root_of "$app")#g" \
    -e "s#\"dnsresolver\": *\"[^\"]*\"#\"dnsresolver\": \"http://directory.localhost:$(router_port)/_dev/dns-query\"#" "$file"
  "$(mesail_root)/deploy/lambda/rewrite-config.sh" "$file" "$MESAIL_ISSUER_ORIGIN" "$(native_keycloak_url)" "port:$(router_port)"
}

router_port() {
  cat "$(run_folder)/router-port"
}

add_datasource() {
  local app="$1"
  point_copy_at_this_run "$app"
  if ! owns_fylo "$app" || declares_datasource "$app"; then return 0; fi
  printf 'YON_DATASOURCE_DRIVER=fylo\nYON_DATASOURCE_URL=fylo://%s\n' "$(app_root_of "$app")" >> "$(run_folder)/apps/$app/.env"
}

write_profile() {
  local app="$1"
  printf '{ "server": { "host": "127.0.0.1", "port": %s } }\n' "$(app_port "$app")" > "$(run_folder)/apps/$app/server/application-native.json"
  add_datasource "$app"
  add_blob_store "$app"
  add_secrets "$app"
}

router_base() {
  echo "http://127.0.0.1:$(cat "$(run_folder)/router-port")"
}

copy_env_file() {
  local app="$1"
  : > "$(run_folder)/apps/$app/.env"
  chmod 600 "$(run_folder)/apps/$app/.env"
  if [ ! -f "$(mesail_root)/apps/$app/.env" ]; then return 0; fi
  grep -v -E '^YON_(DATASOURCE_(DRIVER|URL)|ATTACHMENTS_(ADAPTER|FOLDER|LINK_BASE|LINK_SECRET)|MAILIN_BLOBS_(ADAPTER|FOLDER|LINK_BASE|LINK_SECRET)|REALTIME_GATEWAY_(URL|SECRET))=' "$(mesail_root)/apps/$app/.env" \
    | sed -E -e "s#(http://[a-z0-9-]+\.localhost):81(0[1-9]|1[0-9])#\1:$(router_port)#g" \
      -e "s#http://127\.0\.0\.1:81(0[1-9]|1[0-9])#$(router_base)#g" >> "$(run_folder)/apps/$app/.env" || true
}

add_secrets() {
  local app="$1" line
  mesail_load_secrets
  while read -r line; do
    if grep -q "^${line%%=*}=" "$(run_folder)/apps/$app/.env"; then continue; fi
    echo "$line" >> "$(run_folder)/apps/$app/.env"
  done < <(mesail_app_secrets "$(run_folder)/apps/$app")
}

use_secrets() {
  if [ -n "${MESAIL_SECRETS_FILE:-}" ]; then return 0; fi
  MESAIL_SECRETS_FILE="$(run_folder)/secrets"
  mv "$(mesail_generate_secrets)" "$MESAIL_SECRETS_FILE"
  export MESAIL_SECRETS_FILE
}

blob_folder() {
  echo "$(data_folder)/blobs"
}

blob_secret() {
  mesail_load_secrets
  echo "$MESAIL_LINK_SECRET"
}

secret_folder() {
  echo "$(data_folder)/secrets"
}

dkim_selector() {
  jq -r '.directory.dkimselector // "mesail"' "$(mesail_root)/apps/directory/server/application.json"
}

make_dkim_key() {
  local domain="$1" file
  file="$(secret_folder)/dkim/$domain/$(dkim_selector).pem"
  mkdir -p "$(dirname "$file")"
  (umask 077 && openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 -out "$file" 2> /dev/null)
  echo "test DKIM key for $domain (selector $(dkim_selector)) in the run's secrets folder"
}

make_dkim_keys() {
  local domain
  mkdir -p "$(secret_folder)"
  chmod 700 "$(secret_folder)"
  for domain in $(mesail_stack '.hostedDomains[]'); do
    make_dkim_key "$domain"
  done
}

point_mail_out() {
  local file
  file="$(run_folder)/apps/mail-out/server/application.json"
  jq --arg blobs "$(cd "$(blob_folder)" && pwd -P)" --arg secrets "$(cd "$(secret_folder)" && pwd -P)" \
    '.mailout.blobfolder = $blobs | .mailout.secretfolder = $secrets' "$file" > "$file.new"
  mv "$file.new" "$file"
}

add_mail_in_blobs() {
  printf 'YON_MAILIN_BLOBS_ADAPTER=local\nYON_MAILIN_BLOBS_FOLDER=%s\nYON_MAILIN_BLOBS_LINK_BASE=%s/blobs\n' \
    "$(cd "$(blob_folder)" && pwd -P)" "$(router_base)" >> "$(run_folder)/apps/mail-in/.env"
}

add_mail_blobs() {
  local app="$1"
  mkdir -p "$(blob_folder)"
  case "$app" in
    mail-out) point_mail_out ;;
    mail-in) add_mail_in_blobs ;;
  esac
}

gateway_port() {
  cat "$(run_folder)/gateway-$1-port"
}

add_gateway_address() {
  local app="$1"
  if [ "$app" != realtime ]; then return 0; fi
  echo "YON_REALTIME_GATEWAY_URL=http://127.0.0.1:$(gateway_port manage)" >> "$(run_folder)/apps/realtime/.env"
}

realtime_upstream() {
  sed -n 's/^realtime=//p' "$(run_folder)/upstreams"
}

start_gateway() {
  if [ -z "$(realtime_upstream)" ]; then echo "realtime is not running; no gateway"; return 0; fi
  (cd "$(mesail_root)/edge/realtime" && GOFLAGS=-buildvcs=false go build -o "$(run_folder)/realtime-gateway" .)
  mesail_load_secrets
  MESAIL_REALTIME_GATEWAY_SECRET="$MESAIL_REALTIME_GATEWAY_SECRET" MESAIL_REALTIME_BACKEND="$(realtime_upstream)" \
    MESAIL_REALTIME_ORIGINS="$(router_base)" MESAIL_REALTIME_LISTEN="127.0.0.1:$(gateway_port public)" \
    MESAIL_REALTIME_MANAGE_LISTEN="127.0.0.1:$(gateway_port manage)" \
    "$(run_folder)/realtime-gateway" > "$(mesail_log_dir)/realtime-gateway.log" 2>&1 &
  echo "$!" >> "$(run_folder)/pids"
  echo "realtime gateway on 127.0.0.1:$(gateway_port public), management 127.0.0.1:$(gateway_port manage) (pid $!)"
}

passthrough_routes() {
  local own="" public
  if [ -n "$(realtime_upstream)" ]; then
    public="http://127.0.0.1:$(gateway_port public)"
    own="/api/v1/realtime/socket=$public,/api/v1/realtime/stream=$public"
  fi
  echo "$own${MESAIL_ROUTER_PASSTHROUGH:+,$MESAIL_ROUTER_PASSTHROUGH}" | sed 's/^,//'
}

add_blob_store() {
  local app="$1"
  add_gateway_address "$app"
  add_mail_blobs "$app"
  if [ "$app" != attachments ]; then return 0; fi
  mkdir -p "$(blob_folder)"
  printf 'YON_ATTACHMENTS_ADAPTER=local\nYON_ATTACHMENTS_FOLDER=%s\nYON_ATTACHMENTS_LINK_BASE=%s/blobs\n' \
    "$(cd "$(blob_folder)" && pwd -P)" "$(router_base)" >> "$(run_folder)/apps/$app/.env"
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

app_port() {
  cat "$(run_folder)/port-$1"
}

choose_app_ports() {
  local app
  for app in "$@"; do
    free_port > "$(run_folder)/port-$app"
  done
}

start_app() {
  local app="$1" port
  port="$(app_port "$app")"
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

dev_dns_settings() {
  local directory
  directory="$(mesail_root)/apps/directory/server/application.json"
  if [ ! -f "$directory" ]; then return 0; fi
  echo "MESAIL_ROUTER_DEV_DNS_DOMAINS=$(mesail_stack '.hostedDomains | join(",")')"
  echo "MESAIL_ROUTER_DEV_DNS_MAIL_HOST=$(jq -r '.directory.mailhost' "$directory")"
  echo "MESAIL_ROUTER_DEV_DNS_SPF=$(jq -r '.directory.spfmechanism' "$directory")"
}

blob_settings() {
  if ! grep -qE '^(attachments|mail-in) ' "$(run_folder)/servers"; then return 0; fi
  echo "MESAIL_ROUTER_BLOBS_FOLDER=$(cd "$(blob_folder)" && pwd -P)"
  echo "MESAIL_ROUTER_BLOBS_BASE=$(router_base)/blobs"
  echo "MESAIL_ROUTER_BLOBS_SECRET=$(blob_secret)"
}

router_keycloak() {
  if ! keycloak_wanted; then return 0; fi
  keycloak_base
}

start_router() {
  local settings
  (cd "$(mesail_root)/deploy/router" && GOFLAGS=-buildvcs=false go build -o "$(run_folder)/mesail-router" .)
  read -ra settings <<< "$({ blob_settings; dev_dns_settings; } | tr '\n' ' ')"
  env ${settings[@]+"${settings[@]}"} MESAIL_ROUTER_LISTEN="127.0.0.1:$(cat "$(run_folder)/router-port")" \
    MESAIL_ROUTER_UPSTREAMS="$(joined "$(run_folder)/upstreams")" MESAIL_ROUTER_STATIC_FOLDERS="$(joined "$(run_folder)/static")" \
    MESAIL_ROUTER_KEYCLOAK_URL="$(router_keycloak)" MESAIL_ROUTER_PASSTHROUGH="$(passthrough_routes)" \
    "$(run_folder)/mesail-router" > "$(mesail_log_dir)/router.log" 2>&1 &
  echo "$!" >> "$(run_folder)/pids"
  echo "export MESAIL_BASE_URL=$(router_base)" > "$(run_folder)/env"
  echo "router on $(router_base) (pid $!)"
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
  free_port > "$(run_folder)/router-port"
  free_port > "$(run_folder)/gateway-public-port"
  free_port > "$(run_folder)/gateway-manage-port"
  use_secrets
  reset_data
  mesail_step dkim-keys make_dkim_keys
  launch_keycloak
  # shellcheck disable=SC2046
  choose_app_ports $(yon_apps "$@")
  # shellcheck disable=SC2046
  mesail_step prepare-apps prepare_apps $(yon_apps "$@")
  for app in $(yon_apps "$@"); do
    mesail_step "start-$app" start_app "$app"
  done
  bundle_static_apps
  mesail_step start-gateway start_gateway
  mesail_step start-router start_router
  mesail_step wait-services wait_for_services
  finish_keycloak
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

launch_keycloak() {
  if ! keycloak_wanted; then echo "no Keycloak (MESAIL_KEYCLOAK=${MESAIL_KEYCLOAK})"; return 0; fi
  mesail_step keycloak-build build_keycloak
  mesail_step keycloak-start start_keycloak
}

finish_keycloak() {
  if ! keycloak_wanted; then return 0; fi
  mesail_step keycloak-wait mesail_wait_for Keycloak 300 keycloak_answers
  mesail_step keycloak-provision provision_keycloak
  mesail_step tokens write_tokens
  mesail_step seed-directory seed_directory
  mesail_step scheduler start_scheduler
}

native_schedule_paths() {
  local path
  for path in $(mesail_stack '.schedules[] | select(.native) | .path'); do
    if grep -q "^$(echo "$path" | cut -d/ -f4) " "$(run_folder)/servers"; then echo "$path"; fi
  done | paste -sd, -
}

start_scheduler() {
  local paths
  paths="$(native_schedule_paths)"
  if [ -z "$paths" ]; then echo "no scheduled route needs the runner (the apps' own workers trigger the rest)"; return 0; fi
  mesail_load_secrets
  MESAIL_SCHEDULER_TOKEN_URL="$(keycloak_base)/idp/realms/mesail-operators/protocol/openid-connect/token" \
    MESAIL_SCHEDULER_CLIENT_SECRET="$MESAIL_SERVICE_SECRET" MESAIL_SCHEDULER_BASE="$(router_base)" \
    MESAIL_SCHEDULER_PATHS="$paths" MESAIL_SCHEDULER_SECONDS="$(mesail_stack .scheduleSeconds)" \
    "$(mesail_root)/deploy/lambda/scheduler.sh" > "$(mesail_log_dir)/scheduler.log" 2>&1 &
  echo "$!" >> "$(run_folder)/pids"
  echo "scheduler for $paths (pid $!, log scheduler.log)"
}

load_tokens() {
  if keycloak_wanted; then mesail_step tokens write_tokens; fi
  if [ ! -f "$(tokens_file)" ]; then echo "no tokens file: tests that need a token skip"; return 0; fi
  # shellcheck disable=SC1090
  . "$(tokens_file)"
}

fresh_tokens() {
  export MESAIL_SECRETS_FILE="${MESAIL_SECRETS_FILE:-$(run_folder)/secrets}"
  write_tokens
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
    --tokens) fresh_tokens ;;
    --up) shift; echo "native: logs in $(mesail_log_dir)"; heavy_job "$0" --start-only "$@" ;;
    --start-only) shift; start_all "$@" ;;
    --test-only) shift; start_test_stop "$@" ;;
    *) echo "native: logs in $(mesail_log_dir)"; heavy_job "$0" --test-only "$@" ;;
  esac
}

main "$@"
