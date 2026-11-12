#!/usr/bin/env bash
set -euo pipefail
# shellcheck source-path=SCRIPTDIR
. "$(dirname "$0")/common.sh"

load_tokens() {
  local file
  file="$(mktemp)"
  "$(dirname "$0")/tokens.sh" > "$file"
  # shellcheck disable=SC1090
  . "$file"
  rm -f "$file"
  echo "tokens loaded for: $(env | grep -o '^MESAIL_TOKEN_[A-Z]*=.' | cut -d= -f1 | sort | tr '\n' ' ')"
}

wait_for_service() {
  mesail_wait_for "$1 through the router" 180 mesail_service_answers "$MESAIL_ROUTER_URL" "$1"
}

run_blackbox() {
  local filter=()
  if [ -n "${MESAIL_TEST_RUN:-}" ]; then filter=(-run "$MESAIL_TEST_RUN"); fi
  (cd "$(mesail_root)/tests/blackbox" && MESAIL_BASE_URL="$MESAIL_ROUTER_URL" \
    go test -tags blackbox -count=1 -v ${filter[@]+"${filter[@]}"} ./...)
}

main() {
  mesail_load_settings
  mesail_load_secrets
  mesail_step wait-identity wait_for_service identity
  mesail_step wait-directory wait_for_service directory
  mesail_step seed-directory mesail_seed_directory "$MESAIL_ROUTER_URL"
  load_tokens
  mesail_step blackbox run_blackbox
}

main "$@"
