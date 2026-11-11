#!/usr/bin/env bash
set -Eeuo pipefail

REPO_ROOT=$(cd "$(dirname "$0")/../../.." && pwd)
readonly REPO_ROOT
readonly IMAGE=${MESAIL_TEST_IMAGE:-mesail-keycloak:test}
readonly CONTAINER=mesail-keycloak-test
readonly VOLUME=mesail-keycloak-test-data
readonly PORT=${MESAIL_TEST_PORT:-18180}
readonly HEALTH_TIMEOUT=300
readonly STOP_TIMEOUT=30

export MESAIL_PUBLIC_URL=http://localhost:${PORT}
MESAIL_KEYCLOAK_API_SECRET=$(openssl rand -hex 24)
MESAIL_KEYCLOAK_SERVICE_SECRET=$(openssl rand -hex 24)
BOOTSTRAP_ADMIN_PASSWORD=$(openssl rand -hex 16)
export MESAIL_KEYCLOAK_API_SECRET MESAIL_KEYCLOAK_SERVICE_SECRET

step() {
  printf '=== %s %s\n' "$1" "$(date +%H:%M:%S)"
}

remove_test_state() {
  docker rm -f "$CONTAINER" >/dev/null 2>&1 || true
  docker volume rm "$VOLUME" >/dev/null 2>&1 || true
}

build_image() {
  step "build ${IMAGE} (KEYCLOAK_DB=sqlite)"
  docker build --progress=plain --target mesail-keycloak --build-arg KEYCLOAK_DB=sqlite -t "$IMAGE" "$REPO_ROOT"
}

start_container() {
  step "start container on volume ${VOLUME}"
  docker run -d --name "$CONTAINER" --stop-timeout "$STOP_TIMEOUT" \
    -p "127.0.0.1:${PORT}:8180" -v "${VOLUME}:/data/keycloak" \
    -e KC_HTTP_HOST=0.0.0.0 -e MESAIL_PUBLIC_URL -e MESAIL_KEYCLOAK_API_SECRET -e MESAIL_KEYCLOAK_SERVICE_SECRET \
    -e MESAIL_KEYCLOAK_DEV_USERS=true -e KC_BOOTSTRAP_ADMIN_USERNAME=admin -e KC_BOOTSTRAP_ADMIN_PASSWORD="$BOOTSTRAP_ADMIN_PASSWORD" \
    -e MESAIL_APP_COMMAND='sleep infinity' "$IMAGE" >/dev/null
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
  step "oidc check: $1"
  python3 "$REPO_ROOT/deploy/keycloak/test/oidc_check.py"
}

check_import_skipped() {
  docker logs "$CONTAINER" 2>&1 | grep -E "Realm 'mesail' already exists|Import skipped" | tail -1
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
  start_container
  wait_healthy
  check_oidc "first start"
  step "restart the same container"
  docker restart -t "$STOP_TIMEOUT" "$CONTAINER" >/dev/null
  wait_healthy
  check_import_skipped
  check_oidc "after restart"
  step "replace the container, keep the volume"
  docker rm -f "$CONTAINER" >/dev/null
  start_container
  wait_healthy
  check_import_skipped
  check_oidc "new container on the same volume"
  docker exec "$CONTAINER" ls -la /data/keycloak
  check_supervisor_stops_container
  step "done"
}

main "$@"
