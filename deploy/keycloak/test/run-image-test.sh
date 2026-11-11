#!/usr/bin/env bash
set -Eeuo pipefail

REPO_ROOT=$(cd "$(dirname "$0")/../../.." && pwd)
readonly REPO_ROOT
readonly IMAGE=${MESAIL_TEST_IMAGE:-mesail-keycloak:test}
readonly CONTAINER=mesail-keycloak-test
readonly VOLUME=mesail-keycloak-test-data
readonly LEGACY_VOLUME=mesail-keycloak-test-legacy-data
readonly PORT=${MESAIL_TEST_PORT:-18180}
readonly HEALTH_TIMEOUT=300
readonly STOP_TIMEOUT=30

readonly LEGACY_DOMAINS=example.com,example.org,partner.test
readonly ADDED_DOMAIN=added.test

export MESAIL_KEYCLOAK_URL=http://127.0.0.1:${PORT}/idp
MESAIL_IDENTITY_CLIENT_SECRET=$(openssl rand -hex 24)
MESAIL_SERVICE_SECRET=$(openssl rand -hex 24)
MESAIL_ACCOUNT_CLIENT_SECRET=$(openssl rand -hex 24)
MESAIL_KEYCLOAK_DIRECTORY_SECRET=$(openssl rand -hex 24)
BOOTSTRAP_CLIENT_SECRET=$(openssl rand -hex 24)
export MESAIL_IDENTITY_CLIENT_SECRET MESAIL_SERVICE_SECRET MESAIL_ACCOUNT_CLIENT_SECRET MESAIL_KEYCLOAK_DIRECTORY_SECRET

step() {
  printf '=== %s %s\n' "$1" "$(date +%H:%M:%S)"
}

remove_test_state() {
  docker rm -f "$CONTAINER" >/dev/null 2>&1 || true
  docker volume rm "$VOLUME" "$LEGACY_VOLUME" >/dev/null 2>&1 || true
}

build_image() {
  step "build ${IMAGE} (KEYCLOAK_DB=sqlite)"
  docker build --progress=plain --target mesail-keycloak --build-arg KEYCLOAK_DB=sqlite -t "$IMAGE" "$REPO_ROOT"
}

start_container() {
  step "start container on volume $1 ($2)"
  docker run -d --name "$CONTAINER" --stop-timeout "$STOP_TIMEOUT" \
    -p "127.0.0.1:${PORT}:8180" -v "$1:/data/keycloak" \
    -e KC_HTTP_HOST=0.0.0.0 -e MESAIL_KEYCLOAK_URL -e MESAIL_KEYCLOAK_DEV_USERS=true \
    -e MESAIL_IDENTITY_CLIENT_SECRET -e MESAIL_ACCOUNT_CLIENT_SECRET -e MESAIL_KEYCLOAK_DIRECTORY_SECRET \
    "${@:3}" -e MESAIL_APP_COMMAND='sleep infinity' "$IMAGE" >/dev/null
}

start_first_container() {
  start_container "$VOLUME" "first start, one realm, with the bootstrap admin client" \
    -e KC_BOOTSTRAP_ADMIN_CLIENT_ID=bootstrap-admin -e KC_BOOTSTRAP_ADMIN_CLIENT_SECRET="$BOOTSTRAP_CLIENT_SECRET"
}

start_legacy_container() {
  start_container "$LEGACY_VOLUME" "transition: also the old per-domain realms, until L5" \
    -e MESAIL_KEYCLOAK_DOMAINS="$LEGACY_DOMAINS" -e MESAIL_SERVICE_SECRET \
    -e KC_BOOTSTRAP_ADMIN_CLIENT_ID=bootstrap-admin -e KC_BOOTSTRAP_ADMIN_CLIENT_SECRET="$BOOTSTRAP_CLIENT_SECRET"
}

wait_healthy() {
  local deadline=$((SECONDS + HEALTH_TIMEOUT)) status=starting
  until [[ $status == healthy ]]; do
    (( SECONDS < deadline )) || { docker logs --tail 80 "$CONTAINER"; echo "container not healthy after ${HEALTH_TIMEOUT}s"; return 1; }
    sleep 3
    status=$(docker inspect -f '{{.State.Health.Status}}' "$CONTAINER")
  done
  step "healthy after ${SECONDS}s since script start"
}

check_oidc() {
  step "oidc check of the realm mesail: $1"
  KC_BOOTSTRAP_ADMIN_CLIENT_ID=bootstrap-admin KC_BOOTSTRAP_ADMIN_CLIENT_SECRET="$BOOTSTRAP_CLIENT_SECRET" CHECK_LEGACY_REALMS="${2:-false}" \
    python3 "$REPO_ROOT/deploy/keycloak/test/oidc_check.py"
}

check_legacy_realms() {
  step "legacy check of the per-domain realms (until L5)"
  KC_BOOTSTRAP_ADMIN_CLIENT_ID=bootstrap-admin KC_BOOTSTRAP_ADMIN_CLIENT_SECRET="$BOOTSTRAP_CLIENT_SECRET" CHECK_DOMAINS="$LEGACY_DOMAINS" CHECK_ADD_DOMAIN="$ADDED_DOMAIN" MESAIL_KEYCLOAK_DIRECTORY_CLIENT_ID=mesail-directory \
    CHECK_REALM_TEMPLATE="$REPO_ROOT/deploy/keycloak/realm/domain-realm.json" \
    python3 "$REPO_ROOT/deploy/keycloak/test/legacy_realms_check.py"
}

app_started() {
  docker logs "$CONTAINER" 2>&1 | grep -qF 'app started'
}

show_provisioning() {
  local deadline=$((SECONDS + HEALTH_TIMEOUT))
  until app_started; do
    [[ $(docker inspect -f '{{.State.Running}}' "$CONTAINER") == true ]] || { docker logs --tail 20 "$CONTAINER"; echo "the container stopped"; return 1; }
    (( SECONDS < deadline )) || { docker logs --tail 40 "$CONTAINER"; echo "the app did not start after provisioning"; return 1; }
    sleep 2
  done
  docker logs "$CONTAINER" 2>&1 | grep -F '[mesail-keycloak-provision]' | tail -1
}

expect_provisioning() {
  docker logs "$CONTAINER" 2>&1 | grep -F '[mesail-keycloak-provision]' | grep -qF "$1" && return 0
  echo "the provisioning log does not say: $1"
  return 1
}

check_import_skipped() {
  docker logs "$CONTAINER" 2>&1 | grep -E "already exists. Import skipped" | tail -4
}

check_supervisor_stops_container() {
  step "supervisor: end the app process, expect the container to stop"
  docker exec "$CONTAINER" bash -c 'for process in /proc/[0-9]*; do [[ $(tr "\0" " " < "$process/cmdline" 2>/dev/null) == *"sleep infinity " ]] && kill "${process#/proc/}"; done; true'
  timeout 60 docker wait "$CONTAINER"
  docker logs "$CONTAINER" 2>&1 | grep -F '[mesail-supervisor]' | tail -4
}

main() {
  trap remove_test_state EXIT
  remove_test_state
  build_image
  start_first_container
  wait_healthy
  show_provisioning
  expect_provisioning "nothing to provision"
  check_oidc "first start"
  step "restart the same container"
  docker restart -t "$STOP_TIMEOUT" "$CONTAINER" >/dev/null
  wait_healthy
  check_import_skipped
  check_oidc "after restart"
  step "replace the container, keep the volume"
  docker rm -f "$CONTAINER" >/dev/null
  start_container "$VOLUME" "no bootstrap admin client"
  wait_healthy
  show_provisioning
  check_import_skipped
  check_oidc "new container on the same volume"
  docker exec "$CONTAINER" ls -la /data/keycloak
  step "replace the container: transition mode on a new volume"
  docker rm -f "$CONTAINER" >/dev/null
  start_legacy_container
  wait_healthy
  show_provisioning
  expect_provisioning "created master-realm client"
  check_oidc "transition mode" true
  check_legacy_realms
  check_supervisor_stops_container
  step "done"
}

main "$@"
