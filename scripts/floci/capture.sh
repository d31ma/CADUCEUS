#!/usr/bin/env bash
set -euo pipefail
# shellcheck source-path=SCRIPTDIR
. "$(dirname "$0")/common.sh"

usage() {
  cat << 'TEXT'
Usage: scripts/floci/capture.sh <service> <error number>

Saves the state of a FYLO app that answered a store-unavailable number (for
example MS1048), BEFORE any retry or other call to that app
(docs: deploy/floci/README.md "Capture before retry"):
  1. stops the scheduler services and the app's containers (stop, never rm);
  2. writes <log dir>/capture-<time>/: containers.txt, fylo-trace/ (the start
     snapshot and trace logs, when the image has MESAIL_FYLO_TRACE=1),
     root-after.tar and owner-files/ (read through a READ-ONLY mount);
  3. times fylo on a scratch copy /mnt/data/capture-<time> in its own volume
     (stack.json captureVolume, never the app's volume) with no 5 s limit
     (timing.txt), then removes only that copy (the volume stays);
  4. starts the scheduler services again.
TEXT
}

capture_time() {
  date +%H%M%S
}

compose() {
  docker compose --project-directory "$(mesail_root)" "$@"
}

used_profile() {
  cat "$(mesail_log_dir)/compose-profile"
}

container_pattern() {
  echo "^floci-aws-$(mesail_stack .functionPrefix)$1-[0-9a-f]+\$"
}

app_containers() {
  local service="$1" state="$2"
  docker ps ${state:+"$state"} --format '{{.CreatedAt}}|{{.Names}}' | sort | cut -d'|' -f2 | grep -E "$(container_pattern "$service")" || true
}

stop_callers() {
  local profile
  profile="$(used_profile)"
  # shellcheck disable=SC2086
  compose $profile stop scheduler scheduler-daily
}

stop_app() {
  local service="$1" name
  for name in $(app_containers "$service" ""); do
    echo "$(date -u +%H:%M:%S.000Z) stopping $name (never removed)"
    docker stop --time 10 "$name" > /dev/null
  done
}

record_containers() {
  local service="$1" name
  for name in $(app_containers "$service" --all); do
    docker inspect --format '{{.Name}} created={{.Created}} state={{.State.Status}} started={{.State.StartedAt}} finished={{.State.FinishedAt}}' "$name"
  done
}

copy_from_container() {
  local name="$1" file="$2" folder="$3"
  docker cp "$name:/tmp/$file" "$folder/$file" > /dev/null 2>&1 || return 1
  echo "copied $name:/tmp/$file"
}

trace_pids() {
  sed -n 's/.* pid=\([0-9]*\) .*/\1/p' "$1" 2> /dev/null || true
}

copy_trace_logs() {
  local name="$1" folder="$2" pid
  copy_from_container "$name" fylo-trace-starts.log "$folder" || { echo "$name: no fylo trace (image without MESAIL_FYLO_TRACE=1)"; return 0; }
  for pid in $(trace_pids "$folder/fylo-trace-starts.log"); do
    copy_from_container "$name" "fylo-trace-$pid.log" "$folder" || echo "$name: no fylo-trace-$pid.log"
  done
}

copy_container_files() {
  local name="$1" folder="$2/$1"
  mkdir -p "$folder"
  copy_from_container "$name" fylo-trace-start.log "$folder" || echo "$name: no start log"
  copy_from_container "$name" fylo-root-at-start.tar "$folder" || echo "$name: no start snapshot"
  copy_trace_logs "$name" "$folder"
  docker logs --timestamps "$name" > "$folder/container.log" 2>&1 || true
}

copy_trace() {
  local service="$1" folder="$2" name
  for name in $(app_containers "$service" --all); do
    copy_container_files "$name" "$folder"
  done
}

start_snapshot() {
  local service="$1" folder="$2" name
  for name in $(app_containers "$service" --all | sed '1!G;h;$!d'); do
    if [ -s "$folder/$name/fylo-root-at-start.tar" ]; then echo "$folder/$name/fylo-root-at-start.tar"; return; fi
  done
}

share_name() {
  mesail_service_field "$1" 'fileSystemShare // .name'
}

access_point_id() {
  local token
  token="$(mesail_stack .fileSystem.creationToken)-$(share_name "$1")"
  mesail_aws efs describe-access-points --output text --query "AccessPoints[?ClientToken=='$token'].AccessPointId | [0]"
}

app_volume() {
  local service="$1" access_point found
  access_point="$(access_point_id "$service")"
  found="$(docker volume ls --quiet --filter "name=efs-$access_point-")"
  if [ "$(grep -c . <<< "$found")" != 1 ]; then
    echo "want exactly one volume for access point $access_point, found: ${found:-none}" >&2
    return 1
  fi
  echo "$found"
}

app_image() {
  echo "$(mesail_stack .imagePrefix)$1:$MESAIL_IMAGE_TAG"
}

save_root_after() {
  local service="$1" volume="$2" folder="$3"
  docker run --rm --network none --user 0:0 --volume "$volume:/data:ro" --entrypoint /bin/sh "$(app_image "$service")" \
    -c 'cd /data && tar -cf - "$1" $(find . -maxdepth 1 -name ".$1.fylo-root-owner.lock*" | sed "s|^\./||")' _ "$service" \
    > "$folder/root-after.tar"
  echo "root-after.tar: $(wc -c < "$folder/root-after.tar" | tr -d ' ') bytes, $(tar -tf "$folder/root-after.tar" | grep -c .) entries"
}

save_owner_files() {
  local service="$1" folder="$2"
  mkdir -p "$folder/owner-files"
  tar -xf "$folder/root-after.tar" -C "$folder/owner-files" --exclude "$service" --exclude "$service/*"
  ls -la "$folder/owner-files"
}

fylo_environment_file() {
  local service="$1" file name
  file="$(mktemp "${TMPDIR:-/tmp}/mesail-fylo-env.XXXXXX")"
  chmod 600 "$file"
  name="$(app_containers "$service" --all | tail -1)"
  docker inspect --format '{{range .Config.Env}}{{println .}}{{end}}' "$name" | grep '^FYLO_' > "$file" || true
  echo "$file"
}

snapshot_label() {
  if [ "$1" = "$2/root-after.tar" ]; then echo "root-after.tar (no start snapshot; state AFTER the slow call)"; return; fi
  echo "start snapshot $(basename "$(dirname "$1")")/fylo-root-at-start.tar (state at container start)"
}

scratch_volume() {
  mesail_stack .captureVolume
}

prepare_scratch_volume() {
  local service="$1"
  docker volume create "$(scratch_volume)" > /dev/null
  docker run --rm --network none --user 0:0 --volume "$(scratch_volume):/mnt/data" --entrypoint chown \
    "$(app_image "$service")" 10001:10001 /mnt/data
}

time_fylo() {
  local service="$1" source="$2" scratch="$3" environment
  prepare_scratch_volume "$service" || return 1
  environment="$(fylo_environment_file "$service")"
  docker run --rm -i --network none --user 10001:10001 --env-file "$environment" --volume "$(scratch_volume):/mnt/data" \
    --entrypoint /bin/bash "$(app_image "$service")" \
    -c "$(cat "$(dirname "$0")/capture-timing.sh")" capture-timing "$service" "$scratch" \
    "$(jq -c --arg service "$service" '.captureSteps[$service] // []' "$(mesail_stack_file)")" < "$source" || true
  rm -f "$environment"
}

write_timing() {
  local service="$1" folder="$2" stamp="$3" source
  source="$(start_snapshot "$service" "$folder/fylo-trace")"
  source="${source:-$folder/root-after.tar}"
  {
    echo "capture $stamp of $service; timed on: $(snapshot_label "$source" "$folder")"
    echo "same image $(app_image "$service"), same fylo options as TACHYON 26.46.02, the app's FYLO_ variables (values not shown), no 5 s limit"
    echo "scratch root in the volume $(scratch_volume) (ext4 data disk of the dev host), never in the app's volume"
    time_fylo "$service" "$source" "capture-$stamp"
  } > "$folder/timing.txt" 2>&1
  cat "$folder/timing.txt"
}

start_callers() {
  local profile
  profile="$(used_profile)"
  # shellcheck disable=SC2086
  compose $profile start scheduler scheduler-daily
}

capture() {
  local service="$1" number="$2" stamp folder volume
  stamp="$(capture_time)"
  folder="$(mesail_log_dir)/capture-$stamp"
  mkdir -p "$folder/fylo-trace"
  echo "capture: $service answered $number; saving its state in $folder BEFORE any retry"
  mesail_step "capture-$stamp-stop-callers" stop_callers || echo "warning: the scheduler did not stop"
  mesail_step "capture-$stamp-stop-app" stop_app "$service"
  record_containers "$service" > "$folder/containers.txt"
  mesail_step "capture-$stamp-trace" copy_trace "$service" "$folder/fylo-trace"
  volume="$(app_volume "$service")" || return 1
  echo "volume: $volume (read only for root-after.tar)" | tee "$folder/volume.txt"
  mesail_step "capture-$stamp-root" save_root_after "$service" "$volume" "$folder" || return 1
  mesail_step "capture-$stamp-owner-files" save_owner_files "$service" "$folder"
  mesail_step "capture-$stamp-timing" write_timing "$service" "$folder" "$stamp"
  echo "capture: done, folder $folder (keep the tars until FYLO says they can go)"
}

main() {
  case "${1:-}" in
    ''|-h|--help) usage; return ;;
  esac
  mesail_load_settings
  mesail_load_secrets
  mesail_load_compose_settings
  capture "$1" "${2:-unknown}" || echo "warning: the capture stopped early (see above)"
  mesail_step capture-start-callers start_callers
}

main "$@"
