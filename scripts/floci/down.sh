#!/usr/bin/env bash
set -euo pipefail
# shellcheck source-path=SCRIPTDIR
. "$(dirname "$0")/common.sh"

compose() {
  docker compose --project-directory "$(mesail_root)" --file "$(mesail_root)/docker-compose.yml" "$@"
}

used_profile() {
  cat "$(mesail_log_dir)/compose-profile" 2> /dev/null || true
}

function_containers() {
  docker ps --all --quiet --filter network=mesail_floci_backend
}

remove_function_containers() {
  local containers
  containers="$(function_containers)"
  if [ -z "$containers" ]; then echo "no function containers left"; return; fi
  # shellcheck disable=SC2086
  docker rm --force $containers
}

remove_file_system_volumes() {
  local volumes
  volumes="$(docker volume ls --quiet --filter name=floci-aws-efs)"
  if [ -z "$volumes" ]; then echo "no file system volumes left"; return; fi
  # shellcheck disable=SC2086
  docker volume rm $volumes
}

own_floci() {
  grep -q -- '--profile floci' "$(mesail_log_dir)/compose-profile" 2> /dev/null
}

main() {
  local profile
  mesail_load_settings
  profile="$(used_profile)"
  # shellcheck disable=SC2086
  mesail_step compose-down compose $profile down --volumes --remove-orphans
  if ! own_floci; then echo "Floci belongs to the dev host; its functions and volumes stay"; return; fi
  mesail_step remove-functions remove_function_containers
  mesail_step remove-volumes remove_file_system_volumes
  mesail_step remove-network docker network rm --force mesail_floci_backend
}

main "$@"
