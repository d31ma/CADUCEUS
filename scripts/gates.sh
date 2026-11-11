#!/usr/bin/env bash
set -uo pipefail

repository_root() {
  cd "$(dirname "$0")/.." && pwd
}

tachyon_binary() {
  local pinned="$HOME/Developer/Tools/tachyon-26.46.02/ty"
  if [ -n "${MESAIL_TY:-}" ]; then echo "$MESAIL_TY"; return; fi
  if [ -x "$pinned" ]; then echo "$pinned"; return; fi
  command -v ty
}

log_folder() {
  local base="${LOG_ROOT:-$HOME/Library/Logs/local}"
  local folder
  folder="$base/CADUCEUS/gates-$(date +%Y%m%d-%H%M%S)"
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

check_versions() {
  local ty="$1"
  "$ty" --version | grep -qx '26.46.02' || { echo "ty must be 26.46.02, found $("$ty" --version)"; return 1; }
  go version | grep -Eq 'go1\.(2[7-9]|[3-9][0-9])' || { echo "Go 1.27 or newer is required"; return 1; }
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

selected_apps() {
  local root="$1" only="$2"
  if [ -n "$only" ]; then echo "$root/apps/$only"; return; fi
  find "$root/apps" -mindepth 1 -maxdepth 1 -type d | sort
}

main() {
  local root ty folder only="" app status=0
  [ "${1:-}" = --app ] && only="${2:-}"
  root="$(repository_root)"
  ty="$(tachyon_binary)"
  folder="$(log_folder)"
  echo "gates: logs in $folder"
  run_step "versions" "$folder/versions.log" check_versions "$ty" || return 1
  run_step "doctor" "$folder/doctor.log" "$ty" doctor "$root/apps/web" || status=1
  run_step "gofmt plain go" "$folder/gofmt.log" plain_go_formatted "$root" || status=1
  run_step "gofmt role files" "$folder/gofmt.log" role_files_formatted "$root" || status=1
  run_step "kit sync" "$folder/kit-sync.log" "$root/scripts/kit-sync.sh" --check || status=1
  run_step "kit vet test crap" "$folder/kit.log" kit_checks "$root" "$folder/kit.cover" || status=1
  run_step "tools vet" "$folder/tools.log" sh -c "cd '$root/tools/crap' && go vet ./..." || status=1
  run_step "blackbox compiles" "$folder/blackbox.log" blackbox_compiles "$root" || status=1
  for app in $(selected_apps "$root" "$only"); do
    app_gates "$ty" "$app" "$folder/$(basename "$app").log" || status=1
  done
  echo "=== gates rc=$status"
  return "$status"
}

main "$@"
