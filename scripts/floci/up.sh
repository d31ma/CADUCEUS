#!/usr/bin/env bash
set -euo pipefail
# shellcheck source-path=SCRIPTDIR
. "$(dirname "$0")/common.sh"

floci_ready() {
  curl --fail --silent "$MESAIL_FLOCI_URL/_floci/init" | grep -q '"ready" *: *true'
}

keycloak_ready() {
  curl --fail --silent "$MESAIL_KEYCLOAK_URL/realms/master" > /dev/null
}

router_ready() {
  curl --fail --silent "$MESAIL_ROUTER_URL/_router/ready" > /dev/null
}

compose() {
  docker compose --project-directory "$(mesail_root)" --file "$(mesail_root)/docker-compose.yml" "$@"
}

own_floci_profile() {
  if floci_ready; then echo "Floci already answers at $MESAIL_FLOCI_URL; using it" >&2; return; fi
  echo "--profile floci"
}

main() {
  local profile
  mesail_load_settings
  profile="$(own_floci_profile)"
  echo "$profile" > "$(mesail_log_dir)/compose-profile"
  # shellcheck disable=SC2086
  mesail_step compose-up compose $profile up --detach
  mesail_step wait-floci mesail_wait_for Floci 120 floci_ready
  mesail_step wait-keycloak mesail_wait_for Keycloak 240 keycloak_ready
  mesail_step wait-router mesail_wait_for router 60 router_ready
}

main "$@"
