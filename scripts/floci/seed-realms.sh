#!/usr/bin/env bash
set -euo pipefail
# shellcheck source-path=SCRIPTDIR
. "$(dirname "$0")/common.sh"

admin_token() {
  curl --fail --silent --show-error "$MESAIL_KEYCLOAK_URL/realms/master/protocol/openid-connect/token" \
    --data-urlencode grant_type=password --data-urlencode client_id=admin-cli \
    --data-urlencode "username=$MESAIL_KEYCLOAK_ADMIN" --data-urlencode "password=$MESAIL_KEYCLOAK_ADMIN_PASSWORD" \
    | jq -r .access_token
}

realm_json() {
  jq --arg realm "$1" --arg public "$MESAIL_PUBLIC_URL" --arg password "$MESAIL_SEED_PASSWORD" \
    --arg secret "$MESAIL_SERVICE_SECRET" --arg identitySecret "$MESAIL_IDENTITY_SECRET" --from-file "$(mesail_root)/deploy/floci/realm.jq" "$(mesail_stack_file)"
}

admin_call() {
  local method="$1" path="$2"
  shift 2
  curl --fail --silent --show-error --request "$method" --header "Authorization: Bearer $MESAIL_ADMIN_TOKEN" \
    "$MESAIL_KEYCLOAK_URL/admin/realms$path" "$@"
}

remove_old_realm() {
  local realm="$1"
  if ! admin_call GET "/$realm" --output /dev/null 2> /dev/null; then return 0; fi
  admin_call DELETE "/$realm"
  echo "removed old realm $realm"
}

create_realm() {
  local realm="$1"
  remove_old_realm "$realm"
  realm_json "$realm" | admin_call POST "" --header 'Content-Type: application/json' --data-binary @-
  echo "created realm $realm"
}

main() {
  local realm
  mesail_load_settings
  MESAIL_ADMIN_TOKEN="$(admin_token)"
  for realm in $(mesail_stack '.realms[].name'); do
    mesail_step "seed-$realm" create_realm "$realm"
  done
}

main "$@"
