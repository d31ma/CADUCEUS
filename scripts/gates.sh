#!/usr/bin/env bash
set -uo pipefail

source "$(cd "$(dirname "$0")" && pwd)/ci/tool-lock.sh"

repository_root() {
  cd "$(dirname "$0")/.." && pwd
}

pinned_binary() {
  local tool="$1" override="$2" pinned
  pinned="$HOME/Developer/Tools/$3-$(tool_lock_version "$tool")/$tool"
  if [ -n "$override" ]; then echo "$override"; return; fi
  if [ -x "$pinned" ]; then echo "$pinned"; return; fi
  command -v "$tool"
}

tachyon_binary() {
  pinned_binary ty "${MESAIL_TY:-}" tachyon
}

fylo_binary() {
  pinned_binary fylo "${MESAIL_FYLO:-}" fylo
}

log_folder() {
  local base="${LOG_ROOT:-$HOME/Library/Logs/local}"
  local folder
  folder="$base/CADUCEUS/gates-$(date +%Y%m%d-%H%M%S)-$$"
  mkdir -p "$folder"
  echo "$folder"
}

run_step() {
  local name="$1" log="$2"
  shift 2
  local code
  echo "=== $name $(date +%H:%M:%S)"
  "$@" >> "$log" 2>&1
  code=$?
  echo "=== $name rc=$code"
  [ "$code" = 0 ] || tail -40 "$log"
  return "$code"
}

pinned_tool_matches() {
  local tool="$1" binary="$2" expected found
  expected="$(tool_lock_version "$tool")"
  [ -n "$binary" ] || { echo "$tool $expected is required and was not found"; return 1; }
  found="$("$binary" --version)"
  [ "$found" = "$expected" ] || { echo "$tool must be $expected, found $found ($binary)"; return 1; }
  tool_lock_verify_binary "$tool" "$binary" && echo "$tool $found $binary checksum ok"
}

check_versions() {
  local ty="$1" fylo="$2"
  pinned_tool_matches ty "$ty" || return 1
  pinned_tool_matches fylo "$fylo" || return 1
  go version | grep -Eq 'go1\.(2[7-9]|[3-9][0-9])' || { echo "Go 1.27 or newer is required"; return 1; }
}

use_pinned_tools() {
  local ty="$1" fylo="$2"
  PATH="$(dirname "$ty"):$(dirname "$fylo"):$PATH"
  export PATH
}

plain_go_formatted() {
  local root="$1" unformatted
  unformatted="$(cd "$root" && gofmt -l kit tools tests)"
  [ -z "$unformatted" ] || { echo "gofmt needed: $unformatted"; return 1; }
}

role_file_formatted() {
  local file="$1" difference
  difference="$({ printf 'package main\n\n'; cat "$file"; } | gofmt -d 2>&1)"
  [ -z "$difference" ] || { echo "gofmt needed in $file"; echo "$difference"; return 1; }
}

role_files_formatted() {
  local root="$1" file status=0
  while IFS= read -r file; do
    role_file_formatted "$file" || status=1
  done < <(find "$root/apps" -name '*.go' -not -path '*/server/packages/*' -not -path '*/.tachyon/*' -not -path '*/dist/*')
  return "$status"
}

kit_checks() {
  local root="$1" profile="$2"
  (cd "$root/kit" && go vet ./... && go test -coverprofile="$profile" ./...) && crap_from_tool "$root" "$profile"
}

crap_from_tool() {
  local root="$1" profile="$2" binary
  binary="$(mktemp -d)/crap"
  (cd "$root/tools/crap" && go build -o "$binary" .) && (cd "$root/kit" && "$binary" -profile "$profile" -max 15)
}

blackbox_compiles() {
  local root="$1"
  (cd "$root/tests/blackbox" && go vet -tags blackbox ./...)
}

app_kind_has_client() {
  [ -d "$1/client" ]
}

app_gates() {
  local ty="$1" app="$2" log="$3"
  local name status=0
  name="$(basename "$app")"
  run_step "$name migrate check" "$log" "$ty" migrate check "$app" || status=1
  run_step "$name test" "$log" "$ty" test "$app" --coverage || status=1
  if app_kind_has_client "$app"; then
    run_step "$name bundle" "$log" "$ty" bundle "$app" --target web || status=1
  fi
  return "$status"
}

existing_app() {
  local app="$1"
  [ -d "$app" ] || { echo "gates: no app at $app" >&2; return 1; }
  echo "$app"
}

selected_apps() {
  local root="$1" only="$2"
  if [ -n "$only" ]; then existing_app "$root/apps/$only"; return; fi
  find "$root/apps" -mindepth 1 -maxdepth 1 -type d | sort
}

shared_gates() {
  local root="$1" ty="$2" folder="$3" status=0
  run_step "doctor" "$folder/doctor.log" "$ty" doctor "$root/apps/web" || status=1
  run_step "gofmt plain go" "$folder/gofmt.log" plain_go_formatted "$root" || status=1
  run_step "gofmt role files" "$folder/gofmt.log" role_files_formatted "$root" || status=1
  run_step "kit sync" "$folder/kit-sync.log" "$root/scripts/kit-sync.sh" --check || status=1
  run_step "kit vet test crap" "$folder/kit.log" kit_checks "$root" "$folder/kit.cover" || status=1
  run_step "tools vet" "$folder/tools.log" sh -c "cd '$root/tools/crap' && go vet ./..." || status=1
  run_step "blackbox compiles" "$folder/blackbox.log" blackbox_compiles "$root" || status=1
  return "$status"
}

all_app_gates() {
  local root="$1" ty="$2" folder="$3" only="$4" apps app status=0
  apps="$(selected_apps "$root" "$only")" || return 1
  for app in $apps; do
    app_gates "$ty" "$app" "$folder/$(basename "$app").log" || status=1
  done
  return "$status"
}

option_value() {
  local name="$1"
  shift
  while [ "$#" -gt 0 ]; do
    [ "$1" = "$name" ] && { echo "${2:-}"; return; }
    shift
  done
}

has_flag() {
  local name="$1" argument
  shift
  for argument in "$@"; do
    [ "$argument" = "$name" ] && return 0
  done
  return 1
}

has_unknown_option() {
  local argument previous=""
  for argument in "$@"; do
    case "$argument" in
      --app | --shared-only | --apps-only | --list-apps) ;;
      *) [ "$previous" = --app ] || { echo "gates: unknown option $argument" >&2; return 0; } ;;
    esac
    previous="$argument"
  done
  return 1
}

usage() {
  echo "usage: scripts/gates.sh [--app <name>] [--shared-only | --apps-only] | --list-apps" >&2
  return 2
}

list_apps() {
  local root="$1" app
  for app in $(selected_apps "$root" ""); do
    basename "$app"
  done
}

wanted() {
  local skip_flag="$1"
  shift
  ! has_flag "$skip_flag" "$@"
}

run_gates() {
  local root="$1" folder="$2" only="$3"
  shift 3
  local ty fylo status=0
  ty="$(tachyon_binary)"
  fylo="$(fylo_binary)"
  run_step "versions" "$folder/versions.log" check_versions "$ty" "$fylo" || return 1
  use_pinned_tools "$ty" "$fylo"
  if wanted --apps-only "$@"; then shared_gates "$root" "$ty" "$folder" || status=1; fi
  if wanted --shared-only "$@"; then all_app_gates "$root" "$ty" "$folder" "$only" || status=1; fi
  return "$status"
}

main() {
  local root folder only status
  if has_unknown_option "$@"; then usage; return; fi
  root="$(repository_root)"
  if has_flag --list-apps "$@"; then list_apps "$root"; return; fi
  only="$(option_value --app "$@")"
  folder="$(log_folder)"
  echo "gates: logs in $folder"
  run_gates "$root" "$folder" "$only" "$@"
  status=$?
  echo "=== gates rc=$status"
  return "$status"
}

main "$@"
