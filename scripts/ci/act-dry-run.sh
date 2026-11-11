#!/usr/bin/env bash
set -euo pipefail

run_job() {
  local job="$1"
  shift
  local rc=0
  echo "=== act $job $(date +%T)"
  act workflow_dispatch -W .github/workflows/ci.yml -j "$job" --concurrent-jobs "${MESAIL_ACT_JOBS:-2}" "$@" || rc=$?
  echo "=== act $job rc=$rc $(date +%T)"
  return "$rc"
}

main() {
  cd "$(dirname "$0")/../.."
  run_job workflow-policy "$@"
  run_job shared "$@"
  run_job apps "$@"
}

main "$@"
