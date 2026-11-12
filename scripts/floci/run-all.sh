#!/usr/bin/env bash
set -euo pipefail
# shellcheck source-path=SCRIPTDIR
. "$(dirname "$0")/common.sh"

usage() {
  cat << 'TEXT'
Usage: scripts/floci/run-all.sh [service ...]

Builds the Tac apps and every Lambda image, starts Floci, Keycloak and the router,
deploys the functions, seeds the directory (people from stack.json) and runs the
blackbox tests, then removes the stack. With service names, only those functions are built and deployed.

Settings (environment):
  MESAIL_OWNER       lock owner and log folder name (default: floci-run-all)
  MESAIL_TEST_RUN    go test -run filter for tests/blackbox
  MESAIL_SKIP_STATIC 1 skips the Tac bundles and the static upload
  MESAIL_SKIP_BUILD  1 reuses the images already built
  MESAIL_VM          mesail (default: the mesail dev host and its Floci) or colima
Logs: ~/Library/Logs/local/CADUCEUS/<owner>/floci-<time>/
TEXT
}

inside_lock() {
  local holder="${MAC_HEAVY_JOB##* pid=}"
  [[ "$holder" =~ ^[0-9]+$ ]] && kill -0 "$holder" 2> /dev/null
}

heavy_job() {
  if inside_lock; then "$@"; return; fi
  MAC_HEAVY_COORDINATOR=caduceus-build "$HOME/.local/bin/mac-heavy" --owner "$MESAIL_OWNER" -- "$@"
}

docker_job() {
  if inside_lock; then "$@"; return; fi
  locked_docker_job "$@"
}

locked_docker_job() {
  local lock
  read -ra lock <<< "$(engine_lock)"
  TACHYON_LOCK_ROOT="$HOME/Library/Caches/mac-locks/mesail-docker" MAC_HEAVY_COORDINATOR=mesail-docker \
    "$HOME/.local/bin/mac-heavy" --owner "$MESAIL_OWNER" "${lock[@]}" -- "$@"
}

engine_lock() {
  if [ "${MESAIL_VM:-mesail}" = colima ]; then echo "--vm=colima"; return; fi
  echo "--weight 1"
}

ty_binary() {
  if [ -n "${MESAIL_TY:-}" ]; then echo "$MESAIL_TY"; return; fi
  echo "$HOME/Developer/Tools/tachyon-26.46.02/ty"
}

bundle_static_apps() {
  local app
  for app in $(mesail_stack '.staticApps[].app'); do
    mesail_step "bundle-$app" "$(ty_binary)" bundle "$(mesail_root)/apps/$app" --target web || echo "warning: $app did not bundle; its pages are not served"
  done
}

bundle_step() {
  if [ "${MESAIL_SKIP_STATIC:-0}" = 1 ]; then echo "Tac bundles skipped (MESAIL_SKIP_STATIC=1)"; return; fi
  heavy_job "$0" --bundle-only
}

main() {
  export MESAIL_OWNER="${MESAIL_OWNER:-floci-run-all}"
  mesail_load_settings
  case "${1:-}" in
    -h|--help) usage; return ;;
    --bundle-only) bundle_static_apps; return ;;
  esac
  echo "run-all: logs in $(mesail_log_dir)"
  bundle_step
  docker_job "$(dirname "$0")/stack.sh" "$@"
}

main "$@"
