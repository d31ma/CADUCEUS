#!/usr/bin/env bash
set -euo pipefail
# shellcheck source-path=SCRIPTDIR
. "$(dirname "$0")/common.sh"

minimum_engine_major() {
  echo 26
}

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

share_client_token() {
  echo "$(mesail_stack .fileSystem.creationToken)-$(mesail_service_field attachments 'fileSystemShare // .name')"
}

access_point_id() {
  mesail_aws efs describe-access-points --output text \
    --query "AccessPoints[?ClientToken=='$(share_client_token)'].AccessPointId | [0]"
}

volumes_of() {
  docker volume ls --quiet --filter "name=efs-$1-"
}

wake_attachments() {
  mesail_wait_for "attachments through the router" 180 mesail_service_answers "$MESAIL_ROUTER_URL" attachments
}

volume_after_wake() {
  local access_point="$1"
  echo "warning: no volume for access point $access_point yet; waking attachments once so Floci makes it" >&2
  wake_attachments >&2
  volumes_of "$access_point"
}

one_volume() {
  local access_point="$1" found
  found="$(volumes_of "$access_point")"
  if [ -z "$found" ]; then found="$(volume_after_wake "$access_point")"; fi
  if [ "$(grep -c . <<< "$found")" != 1 ]; then
    echo "want exactly one volume for access point $access_point, found: ${found:-none}" >&2
    return 1
  fi
  echo "$found"
}

blob_volume() {
  local access_point
  access_point="$(access_point_id)"
  if [ -z "$access_point" ] || [ "$access_point" = None ]; then
    echo "no access point with client token $(share_client_token)" >&2
    return 1
  fi
  one_volume "$access_point"
}

check_engine() {
  local version
  version="$(docker version --format '{{.Server.Version}}')"
  if [ "${version%%.*}" -lt "$(minimum_engine_major)" ]; then
    echo "Docker Engine $version cannot mount a volume subpath (needs $(minimum_engine_major) or newer)" >&2
    return 1
  fi
  echo "Docker Engine $version"
}

mount_blobs_on_router() {
  local profile
  profile="$(used_profile)"
  # shellcheck disable=SC2086
  compose $profile --file "$(mesail_root)/deploy/floci/compose.blobs.yaml" up --detach --no-deps blobs-folder router
  mesail_wait_for router 60 curl --fail --silent "$MESAIL_ROUTER_URL/_router/ready"
}

main() {
  mesail_load_settings
  mesail_load_secrets
  mesail_load_compose_settings
  if ! attachments_deployed; then echo "attachments is not deployed; the router serves no /blobs"; return 0; fi
  mesail_step blobs-engine check_engine
  MESAIL_BLOBS_VOLUME="$(blob_volume)"
  export MESAIL_BLOBS_VOLUME
  echo "blob folder: volume $MESAIL_BLOBS_VOLUME, subpath blobs"
  mesail_step blobs-router mount_blobs_on_router
}

main "$@"
