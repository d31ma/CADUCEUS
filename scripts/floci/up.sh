#!/usr/bin/env bash
set -euo pipefail
# shellcheck source-path=SCRIPTDIR
. "$(dirname "$0")/common.sh"

floci_ready() {
  curl --fail --silent "$MESAIL_FLOCI_URL/_floci/init" | grep -q '"ready" *: *true'
}

keycloak_ready() {
  curl --fail --silent "$MESAIL_KEYCLOAK_URL/realms/$(mesail_stack '.keycloak.domains[0]')/.well-known/openid-configuration" > /dev/null
}

router_ready() {
  curl --fail --silent "$MESAIL_ROUTER_URL/_router/ready" > /dev/null
}

realtime_ready() {
  curl --fail --silent "$MESAIL_ROUTER_URL/api/v1/realtime/health" > /dev/null
}

compose() {
  docker compose --project-directory "$(mesail_root)" "$@"
}

compose_options() {
  if [ "${MESAIL_VM:-mesail}" = colima ]; then
    echo "--file $(mesail_root)/docker-compose.yml --profile floci"
    return
  fi
  echo "--file $(mesail_root)/docker-compose.yml --file $(mesail_root)/deploy/floci/compose.devhost.yaml"
}

wait_for_devhost_floci() {
  if [ "${MESAIL_VM:-mesail}" = colima ]; then return 0; fi
  mesail_wait_for "the dev host's Floci at $MESAIL_FLOCI_URL" 120 floci_ready
}

single_writers() {
  mesail_stack '[.services[] | select(.fylo and (.kind // "lambda") == "lambda") | .name] | join(",")'
}

main() {
  local profile
  mesail_load_settings
  mesail_load_secrets
  MESAIL_KEYCLOAK_DOMAINS="$(mesail_stack '.keycloak.domains | join(",")')"
  export MESAIL_KEYCLOAK_DOMAINS
  MESAIL_SINGLE_WRITERS="$(single_writers)"
  MESAIL_SCHEDULER_PATHS="$(mesail_stack '[.schedules[].path] | join(",")')"
  MESAIL_SCHEDULER_SECONDS="$(mesail_stack .scheduleSeconds)"
  MESAIL_DEV_DNS_DOMAINS="$(mesail_stack '.hostedDomains | join(",")')"
  MESAIL_DEV_DNS_MAIL_HOST="$(jq -r '.directory.mailhost' "$(mesail_root)/apps/directory/server/application.json")"
  MESAIL_DEV_DNS_SPF="$(jq -r '.directory.spfmechanism' "$(mesail_root)/apps/directory/server/application.json")"
  export MESAIL_SINGLE_WRITERS MESAIL_SCHEDULER_PATHS MESAIL_SCHEDULER_SECONDS
  export MESAIL_DEV_DNS_DOMAINS MESAIL_DEV_DNS_MAIL_HOST MESAIL_DEV_DNS_SPF
  mesail_step wait-devhost-floci wait_for_devhost_floci
  profile="$(compose_options)"
  echo "$profile" > "$(mesail_log_dir)/compose-profile"
  # shellcheck disable=SC2086
  mesail_step compose-up compose $profile up --detach
  mesail_step wait-floci mesail_wait_for Floci 120 floci_ready
  mesail_step wait-keycloak mesail_wait_for Keycloak 300 keycloak_ready
  mesail_step wait-router mesail_wait_for router 60 router_ready
  mesail_step wait-realtime mesail_wait_for "realtime through the router" 180 realtime_ready
}

main "$@"
