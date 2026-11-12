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
readonly PURGE_PERIOD=60
readonly PURGE_WAIT=150
readonly SLOW_GRANT_SECONDS=2
readonly PURGE_DONE="expiration task completed for realm 'mesail'"
readonly PURGE_FAILED='Unexpected error while removing expired entries'
readonly PURGE_LOG_LEVEL='INFO,org.keycloak.models.jpa.session.UserSessionExpirationLogic:debug'


export MESAIL_KEYCLOAK_URL=http://127.0.0.1:${PORT}/idp
export MESAIL_PASSKEY_RP_ID=mail.example.test
MESAIL_IDENTITY_CLIENT_SECRET=$(openssl rand -hex 24)
MESAIL_ACCOUNT_CLIENT_SECRET=$(openssl rand -hex 24)
MESAIL_KEYCLOAK_DIRECTORY_SECRET=$(openssl rand -hex 24)
BOOTSTRAP_CLIENT_SECRET=$(openssl rand -hex 24)
export MESAIL_IDENTITY_CLIENT_SECRET MESAIL_ACCOUNT_CLIENT_SECRET MESAIL_KEYCLOAK_DIRECTORY_SECRET

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
  step "start container on volume $1 ($2)"
  docker run -d --name "$CONTAINER" --stop-timeout "$STOP_TIMEOUT" \
    -p "127.0.0.1:${PORT}:8180" -v "$1:/data/keycloak" \
    -e KC_HTTP_HOST=0.0.0.0 -e MESAIL_KEYCLOAK_URL -e MESAIL_KEYCLOAK_DEV_USERS=true \
    -e MESAIL_IDENTITY_CLIENT_SECRET -e MESAIL_ACCOUNT_CLIENT_SECRET -e MESAIL_KEYCLOAK_DIRECTORY_SECRET -e MESAIL_PASSKEY_RP_ID \
    "${@:3}" -e MESAIL_APP_COMMAND='sleep infinity' "$IMAGE" >/dev/null
}

start_first_container() {
  start_container "$VOLUME" "first start, with the bootstrap admin client" \
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
  KC_BOOTSTRAP_ADMIN_CLIENT_ID=bootstrap-admin KC_BOOTSTRAP_ADMIN_CLIENT_SECRET="$BOOTSTRAP_CLIENT_SECRET" \
    python3 "$REPO_ROOT/deploy/keycloak/test/oidc_check.py"
}

app_started() {
  docker logs "$CONTAINER" 2>&1 | grep -qF 'app started'
}

wait_for_app() {
  local deadline=$((SECONDS + HEALTH_TIMEOUT))
  until app_started; do
    [[ $(docker inspect -f '{{.State.Running}}' "$CONTAINER") == true ]] || { docker logs --tail 20 "$CONTAINER"; echo "the container stopped"; return 1; }
    (( SECONDS < deadline )) || { docker logs --tail 40 "$CONTAINER"; echo "the app did not start"; return 1; }
    sleep 2
  done
}

check_import_skipped() {
  docker logs "$CONTAINER" 2>&1 | grep -E "already exists. Import skipped" | tail -4
}

token_grant_seconds() {
  curl -s -o /dev/null -w '%{http_code} %{time_total}' --max-time 30 \
    -d client_id=mesail-identity -d client_secret="$MESAIL_IDENTITY_CLIENT_SECRET" -d grant_type=password \
    -d username=ana -d password=dev-ana-password -d scope=openid \
    "${MESAIL_KEYCLOAK_URL}/realms/mesail/protocol/openid-connect/token"
}

purges_finished() {
  docker logs "$CONTAINER" 2>&1 | grep -cF "$PURGE_DONE" || true
}

check_grant() {
  local answer=$1
  [[ ${answer% *} == 200 ]] || { echo "token grant answered ${answer% *}"; return 1; }
  awk -v took="${answer#* }" -v limit="$SLOW_GRANT_SECONDS" 'BEGIN { exit !(took <= limit) }' \
    || { echo "token grant took ${answer#* }s (limit ${SLOW_GRANT_SECONDS}s)"; return 1; }
}

check_session_purge() {
  step "session purge: token grants stay quick while Keycloak removes expired sessions (period ${PURGE_PERIOD}s)"
  local deadline=$((SECONDS + PURGE_WAIT)) grants=0 before
  before=$(purges_finished)
  until (( $(purges_finished) > before )); do
    (( SECONDS < deadline )) || { docker logs --tail 40 "$CONTAINER"; echo "no session purge finished in ${PURGE_WAIT}s"; return 1; }
    check_grant "$(token_grant_seconds)"
    grants=$((grants + 1))
    sleep 0.5
  done
  if docker logs "$CONTAINER" 2>&1 | grep -qF "$PURGE_FAILED"; then echo "the session purge failed"; return 1; fi
  docker logs "$CONTAINER" 2>&1 | grep -F "$PURGE_DONE" | tail -2
  step "session purge finished; ${grants} token grants, none slower than ${SLOW_GRANT_SECONDS}s"
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
  wait_for_app
  check_oidc "first start"
  step "restart the same container"
  docker restart -t "$STOP_TIMEOUT" "$CONTAINER" >/dev/null
  wait_healthy
  check_import_skipped
  check_oidc "after restart"
  step "replace the container, keep the volume"
  docker rm -f "$CONTAINER" >/dev/null
  start_container "$VOLUME" "no bootstrap admin client, session purge every ${PURGE_PERIOD}s" \
    -e KC_SPI_USER_SESSIONS__INFINISPAN__SESSION_EXPIRATION_PERIOD="$PURGE_PERIOD" -e KC_LOG_LEVEL="$PURGE_LOG_LEVEL"
  wait_healthy
  wait_for_app
  check_import_skipped
  check_oidc "new container on the same volume"
  check_session_purge
  docker exec "$CONTAINER" ls -la /data/keycloak
  check_supervisor_stops_container
  step "done"
}

main "$@"
