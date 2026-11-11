#!/usr/bin/env bash
set -uo pipefail

run_act() {
  local name="$1"
  shift
  local rc=0
  echo "=== act $name $(date +%T)"
  act workflow_dispatch -W .github/workflows/ci.yml "$@" || rc=$?
  echo "=== act $name rc=$rc $(date +%T)"
  return "$rc"
}

run_app() {
  local app="$1"
  shift
  run_act "app $app" -j apps --matrix "app:$app" "$@"
}

run_apps() {
  local failed=0
  local app
  for app in $(scripts/gates.sh --list-apps); do
    run_app "$app" "$@" || failed=1
  done
  return "$failed"
}

main() {
  cd "$(dirname "$0")/../.." || exit 1
  local failed=0
  run_act workflow-policy -j workflow-policy "$@" || failed=1
  run_act shared -j shared "$@" || failed=1
  run_apps "$@" || failed=1
  echo "=== act-dry-run rc=$failed"
  return "$failed"
}

main "$@"
