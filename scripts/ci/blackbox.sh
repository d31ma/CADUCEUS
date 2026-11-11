#!/usr/bin/env bash
set -uo pipefail

repository_root() {
  cd "$(dirname "$0")/../.." && pwd
}

stack_runner() {
  local runner="$1/scripts/floci/run-all.sh"
  [ -x "$runner" ] || { echo "blackbox: $runner is missing; it comes with slice P1 (the Floci stack)" >&2; return 1; }
  echo "$runner"
}

main() {
  local root runner code
  root="$(repository_root)"
  runner="$(stack_runner "$root")" || return 1
  echo "=== blackbox $(date +%H:%M:%S) $runner $*"
  "$runner" "$@"
  code=$?
  echo "=== blackbox rc=$code"
  return "$code"
}

main "$@"
