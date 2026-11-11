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
  echo "tokens loaded for: $(env | grep -o '^MESAIL_TOKEN_[A-Z]*' | sort | tr '\n' ' ')"
}

run_blackbox() {
  local filter=()
  if [ -n "${MESAIL_TEST_RUN:-}" ]; then filter=(-run "$MESAIL_TEST_RUN"); fi
  (cd "$(mesail_root)/tests/blackbox" && MESAIL_BASE_URL="$MESAIL_ROUTER_URL" \
    go test -tags blackbox -count=1 -v ${filter[@]+"${filter[@]}"} ./...)
}

main() {
  mesail_load_settings
  load_tokens
  mesail_step blackbox run_blackbox
}

main "$@"
