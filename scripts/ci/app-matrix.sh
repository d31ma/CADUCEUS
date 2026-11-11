#!/usr/bin/env bash
set -euo pipefail

apps_json() {
  "$(cd "$(dirname "$0")/.." && pwd)/gates.sh" --list-apps | awk 'NF { printf "%s\"%s\"", (count++ ? "," : "["), $0 } END { print "]" }'
}

main() {
  local json
  json="$(apps_json)"
  echo "apps=$json"
  echo "apps=$json" >> "${GITHUB_OUTPUT:-/dev/null}"
}

main "$@"
