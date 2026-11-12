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

warm_functions() {
  local service
  for service in $(mesail_lambda_services); do
    wait_for_service "$service" || return 1
  done
}

wait_for_service() {
  mesail_wait_for "$1 through the router" 180 mesail_service_answers "$MESAIL_ROUTER_URL" "$1"
}

go_blackbox() {
  (cd "$(mesail_root)/tests/blackbox" && MESAIL_BASE_URL="$MESAIL_ROUTER_URL" \
    go test -tags blackbox -count=1 -v -run "$1" ./...)
}

test_names() {
  grep -ho '^func Test[A-Za-z0-9_]*' "$1" | sed 's/^func //' | paste -sd'|' -
}

run_file() {
  local file="$1" names
  names="$(test_names "$file")"
  if [ -z "$names" ]; then return 0; fi
  echo "=== tests of $(basename "$file") with fresh tokens"
  load_tokens
  go_blackbox "^(${names})\$"
}

run_blackbox() {
  local file code=0
  if [ -n "${MESAIL_TEST_RUN:-}" ]; then go_blackbox "$MESAIL_TEST_RUN"; return; fi
  for file in "$(mesail_root)"/tests/blackbox/*_test.go; do
    run_file "$file" || code=1
  done
  return "$code"
}

main() {
  mesail_load_settings
  mesail_load_secrets
  mesail_step warm-functions warm_functions
  mesail_step seed-directory mesail_seed_directory "$MESAIL_ROUTER_URL"
  load_tokens
  mesail_step blackbox run_blackbox
}

main "$@"
