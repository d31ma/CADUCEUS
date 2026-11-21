#!/usr/bin/env bash
set -euo pipefail
# shellcheck source-path=SCRIPTDIR
. "$(dirname "$0")/common.sh"

compose() {
  docker compose --project-directory "$(mesail_root)" "$@"
}

base_profile() {
  cat "$(mesail_log_dir)/compose-profile" 2> /dev/null || echo "--file $(mesail_root)/docker-compose.yml"
}

blobs_volume() {
  cat "$(mesail_log_dir)/blobs-volume" 2> /dev/null || true
}

used_profile() {
  if [ -z "$(blobs_volume)" ]; then base_profile; return; fi
  echo "$(base_profile) --file $(mesail_root)/deploy/floci/compose.blobs.yaml"
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

first_line_time() {
  head -1 "$1" 2> /dev/null | cut -d' ' -f1
}

trace_order() {
  local folder="$1" snapshot first
  snapshot="$(first_line_time "$folder/fylo-trace-start.log")"
  first="$(first_line_time "$folder/fylo-trace-starts.log")"
  if [ -z "$snapshot" ]; then echo "no start snapshot line"; return; fi
  if [ -z "$first" ] || [[ "$snapshot" < "$first" ]]; then echo "start snapshot $snapshot before the first fylo start ${first:-none}"; return; fi
  echo "WARNING: first fylo start $first before the start snapshot $snapshot"
}

save_trace() {
  local container="$1" folder
  if [ "$MESAIL_FYLO_TRACE" != 1 ]; then return 0; fi
  folder="$(mesail_log_dir)/fylo-trace/$container"
  mkdir -p "$folder"
  docker cp "$container:/tmp/fylo-trace-start.log" "$folder/" > /dev/null 2>&1 || return 0
  docker cp "$container:/tmp/fylo-trace-starts.log" "$folder/" > /dev/null 2>&1 || true
  echo "fylo trace of $container: $(grep -c . "$folder/fylo-trace-starts.log" 2> /dev/null || echo 0) fylo starts; $(trace_order "$folder")"
}

save_logs() {
  local container
  for container in $(docker ps --all --filter network=mesail_floci_backend --format '{{.Names}}'); do
    docker logs --timestamps "$container" > "$(mesail_log_dir)/container-$container.log" 2>&1 || true
    echo "saved the log of $container"
    save_trace "$container"
  done
}

main() {
  local profile
  mesail_load_settings
  mesail_load_secrets
  MESAIL_BLOBS_VOLUME="$(blobs_volume)"
  export MESAIL_BLOBS_VOLUME
  profile="$(used_profile)"
  mesail_step save-logs save_logs
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
