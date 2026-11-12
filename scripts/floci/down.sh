#!/usr/bin/env bash
set -euo pipefail
# shellcheck source-path=SCRIPTDIR
. "$(dirname "$0")/common.sh"

compose() {
  docker compose --project-directory "$(mesail_root)" "$@"
}

used_profile() {
  cat "$(mesail_log_dir)/compose-profile" 2> /dev/null || echo "--file $(mesail_root)/docker-compose.yml"
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

remove_function() {
  local name="$1"
  if ! mesail_aws lambda get-function --function-name "$name" > /dev/null 2>&1; then return 0; fi
  mesail_aws lambda delete-function --function-name "$name"
  echo "deleted $name"
}

remove_functions_in_floci() {
  local service
  for service in $(mesail_lambda_services); do
    remove_function "$(mesail_stack .functionPrefix)$service"
  done
}

own_floci() {
  [ "${MESAIL_VM:-mesail}" = colima ] && grep -q -- '--profile floci' "$(mesail_log_dir)/compose-profile" 2> /dev/null
}

save_logs() {
  local profile="$1" service
  for service in scheduler router realtime; do
    # shellcheck disable=SC2086
    compose $profile logs --no-color --timestamps "$service" > "$(mesail_log_dir)/container-$service.log" 2>&1 || true
  done
  echo "container logs saved for scheduler, router and realtime"
}

main() {
  local profile
  mesail_load_settings
  mesail_load_secrets
  profile="$(used_profile)"
  mesail_step save-logs save_logs "$profile"
  if ! own_floci; then
    # shellcheck disable=SC2086
    mesail_step compose-stop compose $profile stop
    mesail_step remove-functions remove_functions_in_floci
    return
  fi
  # shellcheck disable=SC2086
  mesail_step compose-down compose $profile down --remove-orphans
  mesail_step remove-functions remove_function_containers
  mesail_step remove-volumes remove_file_system_volumes
  mesail_step remove-network docker network rm --force mesail_floci_backend
}

main "$@"
