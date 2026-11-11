#!/usr/bin/env bash
set -euo pipefail
# shellcheck source-path=SCRIPTDIR
. "$(dirname "$0")/common.sh"

build_image() {
  local target="$1" tag="$2"
  shift 2
  BUILDX_GIT_INFO=false docker --context default buildx build --builder default --load --progress plain \
    --platform "linux/$(mesail_stack .architecture)" \
    --provenance=false --label org.mesail.stack=dev --target "$target" --tag "$tag" "$@" "$(mesail_root)"
}

build_container() {
  local service="$1"
  build_image "$service" "$(mesail_service_field "$service" image):$MESAIL_IMAGE_TAG"
}

build_service() {
  local service="$1"
  if mesail_is_container "$service"; then build_container "$service"; return; fi
  build_image lambda "$(mesail_stack .imagePrefix)$service:$MESAIL_IMAGE_TAG" \
    --build-arg "APP=$service" --build-arg "FYLO=$(mesail_service_field "$service" fylo)"
}

chosen_services() {
  if [ "$#" -gt 0 ]; then printf '%s\n' "$@"; return; fi
  mesail_services
}

main() {
  local service
  mesail_load_settings
  mesail_step build-router build_image router "mesail-router:$MESAIL_IMAGE_TAG"
  mesail_step build-scheduler build_image scheduler "mesail-scheduler:$MESAIL_IMAGE_TAG"
  mesail_step build-keycloak build_image keycloak-blackbox "$(mesail_stack .keycloak.image):$MESAIL_IMAGE_TAG"
  for service in $(chosen_services "$@"); do
    mesail_step "build-$service" build_service "$service" || echo "warning: $service did not build; it is not deployed"
  done
  mesail_step prune-images docker image prune --force --filter label=org.mesail.stack=dev
}

main "$@"
