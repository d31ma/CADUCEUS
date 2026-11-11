#!/usr/bin/env bash
set -euo pipefail
# shellcheck source-path=SCRIPTDIR
. "$(dirname "$0")/common.sh"

user_token() {
  local realm="$1" username="$2"
  curl --fail --silent --show-error "$MESAIL_KEYCLOAK_URL/realms/$realm/protocol/openid-connect/token" \
    --data-urlencode grant_type=password --data-urlencode client_id=mesail-blackbox \
    --data-urlencode "username=$username" --data-urlencode "password=$MESAIL_SEED_PASSWORD" \
    --data-urlencode scope=openid \
    | jq -r .access_token
}

main() {
  local realm username name
  mesail_load_settings
  # shellcheck disable=SC2016
  while read -r realm username name; do
    echo "export MESAIL_TOKEN_$name=$(user_token "$realm" "$username")"
  done < <(mesail_stack '.realms[] | .name as $realm | .users[] | "\($realm) \(.username) \(.token)"')
}

main "$@"
