#!/usr/bin/env bash
set -euo pipefail
# shellcheck source-path=SCRIPTDIR
. "$(dirname "$0")/common.sh"

compose() {
  docker compose --project-directory "$(mesail_root)" "$@"
}

used_profile() {
  cat "$(mesail_log_dir)/compose-profile"
}

attachments_function() {
  echo "$(mesail_stack .functionPrefix)attachments"
}

attachments_deployed() {
  mesail_aws lambda get-function --function-name "$(attachments_function)" > /dev/null 2>&1
}

attachments_container() {
  docker ps --filter "name=floci-aws-$(attachments_function)-" --format '{{.Names}}' | head -n 1
}

data_volume() {
  docker inspect --format "{{range .Mounts}}{{if eq .Destination \"$(mesail_stack .fileSystem.mountPath)\"}}{{.Name}}{{end}}{{end}}" "$1"
}

wake_attachments() {
  mesail_wait_for "attachments through the router" 180 mesail_service_answers "$MESAIL_ROUTER_URL" attachments
}

find_blob_volume() {
  local container
  container="$(attachments_container)"
  if [ -z "$container" ]; then echo "no running attachments container" >&2; return 1; fi
  data_volume "$container"
}

mount_blobs_on_router() {
  local profile
  profile="$(used_profile)"
  # shellcheck disable=SC2086
  compose $profile --file "$(mesail_root)/deploy/floci/compose.blobs.yaml" up --detach --no-deps router
  mesail_wait_for router 60 curl --fail --silent "$MESAIL_ROUTER_URL/_router/ready"
}

main() {
  mesail_load_settings
  mesail_load_secrets
  mesail_load_compose_settings
  if ! attachments_deployed; then echo "attachments is not deployed; the router serves no /blobs"; return 0; fi
  mesail_step blobs-wake-attachments wake_attachments
  MESAIL_BLOBS_VOLUME="$(find_blob_volume)"
  if [ -z "$MESAIL_BLOBS_VOLUME" ]; then echo "the attachments container has no volume at the data mount" >&2; return 1; fi
  export MESAIL_BLOBS_VOLUME
  echo "blob folder: volume $MESAIL_BLOBS_VOLUME, /blobs in it"
  mesail_step blobs-router mount_blobs_on_router
}

main "$@"
