#!/usr/bin/env bash
set -euo pipefail
# shellcheck source-path=SCRIPTDIR
. "$(dirname "$0")/common.sh"

print_person() {
  local username="$1" password="$2" name="$3"
  printf 'export MESAIL_TOKEN_%s=%q\n' "$name" "$(mesail_sign_in "$MESAIL_ROUTER_URL" "$username" "$password" || true)"
  printf 'export MESAIL_PASSWORD_%s=%q\n' "$name" "$password"
}

main() {
  local username password name
  mesail_load_settings
  mesail_load_secrets
  while read -r username password name; do
    print_person "$username" "$password" "$name"
  done < <(mesail_stack '.people[] | "\(.username) \(.password) \(.token)"')
  printf 'export MESAIL_SERVICE_CLIENT_SECRET=%q\n' "$MESAIL_SERVICE_CLIENT_SECRET"
}

main "$@"
