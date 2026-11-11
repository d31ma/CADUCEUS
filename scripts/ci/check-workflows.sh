#!/usr/bin/env bash
set -uo pipefail

repository_root() {
  cd "$(dirname "$0")/../.." && pwd
}

workflow_files() {
  find "$1/.github/workflows" -type f \( -name '*.yml' -o -name '*.yaml' \) | sort
}

report() {
  local rule="$1" findings="$2"
  [ -z "$findings" ] && return 0
  echo "workflow policy: $rule"
  echo "$findings"
  return 1
}

floating_images() {
  report "runner images must be fixed versions, never *-latest" "$(grep -nE '[a-z]+-latest' "$1" | sed "s|^|$1:|")"
}

unfixed_runners() {
  report "runs-on must name a fixed image such as ubuntu-24.04" "$(grep -nE '^ +runs-on:' "$1" | grep -vE 'runs-on: +(ubuntu|macos|windows)-[0-9]' | sed "s|^|$1:|")"
}

unpinned_actions() {
  report "actions must be pinned to a full commit SHA" "$(grep -nE '^ *(- )?uses: ' "$1" | grep -vE "uses: +(\./|[^@ ]+@[0-9a-f]{40}( |$))" | sed "s|^|$1:|")"
}

has_pull_request_trigger() {
  awk '/^on:/ { inside = 1; next } /^[^ ]/ { inside = 0 } inside && /^  pull_request(_target)?:/ { found = 1 } END { exit !found }' "$1"
}

unguarded_jobs() {
  awk -v file="$1" '
    /^jobs:/ { in_jobs = 1; next }
    /^[^ ]/ { in_jobs = 0 }
    in_jobs && /^  [A-Za-z0-9_-]+:/ { if (job != "" && !guarded) print file ": job " job " has no develop/release guard"; job = $1; guarded = 0; next }
    in_jobs && /^    if:/ && index($0, "github.head_ref == '\''develop'\''") && index($0, "startsWith(github.head_ref, '\''release/'\'')") { guarded = 1 }
    END { if (job != "" && !guarded) print file ": job " job " has no develop/release guard" }
  ' "$1"
}

pull_request_guards() {
  has_pull_request_trigger "$1" || return 0
  report "every job of a pull_request workflow must run only for develop and release/* heads" "$(unguarded_jobs "$1")"
}

lint() {
  local root="$1"
  command -v actionlint > /dev/null || { echo "actionlint is required: scripts/ci/install-tools.sh <folder> actionlint"; return 1; }
  (cd "$root" && actionlint -shellcheck= -pyflakes= -verbose)
}

check_file() {
  local file="$1" status=0
  floating_images "$file" || status=1
  unfixed_runners "$file" || status=1
  unpinned_actions "$file" || status=1
  pull_request_guards "$file" || status=1
  return "$status"
}

main() {
  local root file status=0
  root="$(repository_root)"
  for file in $(workflow_files "$root"); do
    echo "=== policy ${file#"$root"/}"
    check_file "$file" || status=1
  done
  echo "=== actionlint"
  lint "$root" || status=1
  echo "=== workflow policy rc=$status"
  return "$status"
}

main "$@"
