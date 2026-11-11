#!/usr/bin/env bash
set -euo pipefail
# shellcheck source-path=SCRIPTDIR
. "$(dirname "$0")/common.sh"

colima_env() {
  echo "$HOME/.local/share/tachyon-ci/docker-linux/colima-env.zsh"
}

use_colima() {
  export DOCKER_HOST="unix:///Volumes/HANGAR/colima/default/docker.sock"
}

devhost() {
  "$HOME/.local/bin/mesail-devhost" "$@"
}

devhost_running() {
  devhost status 2>&1 | grep -q '"status": "running_verified"'
}

start_devhost() {
  if devhost_running; then echo "the mesail dev host is already up"; return; fi
  mesail_step devhost-up devhost up
  MESAIL_STARTED_DEVHOST=1
}

use_devhost() {
  start_devhost
  eval "$(devhost env)"
  export MESAIL_FLOCI_PORT=14566
}

choose_engine() {
  case "${MESAIL_VM:-mesail}" in
    colima) use_colima; start_engine ;;
    mesail) use_devhost ;;
    *) echo "MESAIL_VM must be mesail or colima, not ${MESAIL_VM}"; return 1 ;;
  esac
}

engine_answers() {
  docker info > /dev/null 2>&1
}

start_colima() {
  zsh -c 'source "$1"; colima_start "$2"' _ "$(colima_env)" "$(mesail_log_dir)/colima.log"
}

stop_colima() {
  zsh -c 'source "$1"; colima_stop "$2"' _ "$(colima_env)" "$(mesail_log_dir)/colima.log"
}

start_engine() {
  if engine_answers; then echo "docker engine already running"; return; fi
  mesail_step colima-start start_colima
  MESAIL_STARTED_ENGINE=1
}

finish() {
  local code=$?
  "$(dirname "$0")/down.sh" || code=1
  if [ "${MESAIL_STARTED_ENGINE:-0}" = 1 ]; then mesail_step colima-stop stop_colima || code=1; fi
  if [ "${MESAIL_STARTED_DEVHOST:-0}" = 1 ]; then mesail_step devhost-down devhost down || code=1; fi
  echo "=== stack rc=$code (logs in $(mesail_log_dir))"
  exit "$code"
}

build_images() {
  if [ "${MESAIL_SKIP_BUILD:-0}" = 1 ]; then echo "image build skipped (MESAIL_SKIP_BUILD=1)"; return; fi
  "$(dirname "$0")/build.sh" "$@"
}

main() {
  trap finish EXIT
  choose_engine
  mesail_load_settings
  build_images "$@"
  "$(dirname "$0")/up.sh"
  "$(dirname "$0")/deploy.sh" "$@"
  "$(dirname "$0")/seed-realms.sh"
  "$(dirname "$0")/test.sh"
}

main "$@"
