#!/usr/bin/env bash
set -euo pipefail
# shellcheck source-path=SCRIPTDIR
. "$(dirname "$0")/common.sh"

user_token() {
  local realm="$1" username="$2" password="$3"
  curl --fail --silent --show-error "$MESAIL_KEYCLOAK_URL/realms/$realm/protocol/openid-connect/token" \
    --data-urlencode grant_type=password --data-urlencode client_id=mesail-identity \
    --data-urlencode "client_secret=$MESAIL_IDENTITY_CLIENT_SECRET" \
    --data-urlencode "username=$username" --data-urlencode "password=$password" \
    --data-urlencode scope=openid \
    | jq -r .access_token
}

main() {
  local realm username password name
  mesail_load_settings
  mesail_load_secrets
  while read -r realm username password name; do
    echo "export MESAIL_TOKEN_$name=$(user_token "$realm" "$username" "$password")"
  done < <(mesail_stack '.testUsers[] | "\(.realm) \(.username) \(.password) \(.token)"')
}

main "$@"
