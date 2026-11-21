#!/usr/bin/env bash
set -uo pipefail

source "$(cd "$(dirname "$0")" && pwd)/ci/tool-lock.sh"
# shellcheck source-path=SCRIPTDIR
source "$(cd "$(dirname "$0")" && pwd)/go-cache.sh"
# shellcheck source-path=SCRIPTDIR
source "$(cd "$(dirname "$0")" && pwd)/fylo-image.sh"

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
  [ "$code" = 0 ] || show_failure "$log"
  return "$code"
}

show_failure() {
  grep -E '^ *(--- )?FAIL' "$1" | tail -20
  tail -40 "$1"
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

go_modules() {
  local root="$1"
  (cd "$root" && find . -name go.mod -not -path './apps/*' -not -path '*/vendor/*' -not -path '*/testdata/*' -not -path '*/node_modules/*' -not -path '*/.tachyon/*') | sed 's|^\./||; s|/go\.mod$||' | sort
}

is_production_module() {
  case "$1" in
    tests/* | tools/*) return 1 ;;
  esac
}

module_build_tags() {
  grep -rh '^//go:build [a-z0-9_]*$' . --include='*.go' --exclude-dir=vendor | sed 's|^//go:build ||' | sort -u | paste -sd, -
}

module_vet_options() {
  local tags
  tags="$(module_build_tags)"
  [ -z "$tags" ] || echo "-tags=$tags"
}

module_test_options() {
  local tags
  tags="$(module_build_tags)"
  [ -z "$tags" ] || echo "-tags=$tags -run=^\$"
}

module_formatted() {
  local unformatted
  unformatted="$(find . -name '*.go' -not -path './vendor/*' -exec gofmt -l {} +)"
  [ -z "$unformatted" ] || { echo "gofmt needed: $unformatted"; return 1; }
}

module_vetted() {
  go vet $(module_vet_options) ./...
}

module_reads_repository_files() {
  [ "$1" = tests/kitsync ]
}

module_cache_option() {
  if module_reads_repository_files "$1"; then echo "-count=1"; fi
}

module_tested() {
  local root="$1" module="$2" profile="$3"
  if is_production_module "$module"; then
    go test -coverprofile="$profile" ./... && crap_from_tool "$root" "$profile" "$root/$module"
    return
  fi
  go test $(module_test_options) $(module_cache_option "$module") ./...
}

module_checks() {
  local root="$1" module="$2" profile="$3"
  (cd "$root/$module" && module_formatted && module_vetted && module_tested "$root" "$module" "$profile")
}

module_step_name() {
  local module="$1"
  if is_production_module "$module"; then echo "$module gofmt vet test crap"; return; fi
  echo "$module gofmt vet test"
}

module_gates() {
  local root="$1" folder="$2" module file status=0
  local modules
  modules="$(go_modules "$root")"
  echo "=== go modules: $(echo $modules)"
  for module in $modules; do
    file="$folder/module-${module//\//-}"
    run_step "$(module_step_name "$module")" "$file.log" module_checks "$root" "$module" "$file.cover" || status=1
  done
  return "$status"
}

crap_from_tool() {
  local root="$1" profile="$2" module="$3" binary
  binary="$(mktemp -d)/crap"
  (cd "$root/tools/crap" && go build -o "$binary" .) && (cd "$module" && "$binary" -profile "$profile" -max 15)
}

is_local_package() {
  local packages="$1" name="$2"
  [ "$name" != vendor ] && ! grep -qx "$name" "$packages/kit.list" 2> /dev/null
}

local_packages() {
  local packages="$1" folder name
  for folder in "$packages"/*/; do
    name="$(basename "$folder")"
    if is_local_package "$packages" "$name"; then echo "./$name/..."; fi
  done
}

has_local_packages() {
  local packages="$1/server/packages"
  [ -f "$packages/go.mod" ] && [ -n "$(local_packages "$packages")" ]
}

local_package_checks() {
  local app="$1" profile="$2" packages list
  packages="$app/server/packages"
  list="$(local_packages "$packages" | tr '\n' ' ')"
  echo "app-local packages: $list"
  (cd "$packages" && go vet $list && go test -coverprofile="$profile" $list) && crap_from_tool "$(repository_root)" "$profile" "$packages"
}

app_kind_has_server() {
  [ -d "$1/server" ]
}

atlas_mount_point() {
  echo "$HOME/Developer/AtlasFylo/mesail"
}

worktree_slice() {
  local name
  name="$(basename "$1")"
  case "$name" in
    slice-*) echo "${name#slice-}" ;;
    *) echo "$name" | tr -c 'A-Za-z0-9._\n-' '-' ;;
  esac
}

slice_id() {
  local root="$1"
  if [ -n "${MESAIL_SLICE:-}" ]; then echo "$MESAIL_SLICE"; return; fi
  worktree_slice "$root"
}

fylo_root_base() {
  local root="$1" slice
  if [ -n "${MESAIL_FYLO_ROOT:-}" ]; then echo "$MESAIL_FYLO_ROOT"; return; fi
  slice="$(slice_id "$root")" || return 1
  echo "$(atlas_mount_point)/slice-$slice"
}

uses_fylo() {
  grep -qx 'YON_DATASOURCE_DRIVER=fylo' "$1/.env.example" 2> /dev/null
}

datasource_folder() {
  sed -n 's|^YON_DATASOURCE_URL=fylo://||p' "$1/.env" | tail -1
}

new_app_env() {
  local root="$1" app="$2" base
  base="$(fylo_root_base "$root")" || return 1
  printf 'YON_DATASOURCE_DRIVER=fylo\nYON_DATASOURCE_URL=fylo://%s\n' "$base/$(basename "$app")" > "$app/.env"
  echo "created $app/.env"
}

ensure_app_env() {
  local root="$1" app="$2"
  [ -f "$app/.env" ] && return 0
  new_app_env "$root" "$app"
}

atlas_image_attached() {
  local mount_point
  mount_point="$(atlas_mount_point)"
  MESAIL_FYLO_IMAGE_MOUNT="$mount_point" mesail_fylo_image_ready && return 0
  echo "the FYLO test image is not attached at $mount_point and could not be attached; attach it by hand:"
  echo "diskutil image attach --mountOptions nobrowse --mountPoint $mount_point $(mesail_fylo_image_file)"
  return 1
}

folder_storage_ready() {
  case "$1" in
    "$(atlas_mount_point)"/*) atlas_image_attached ;;
  esac
}

gate_made_root() {
  case "$1" in
    "$(atlas_mount_point)"/slice-*/*) return 0 ;;
  esac
  return 1
}

emptied_fylo_folder() {
  local folder="$1"
  gate_made_root "$folder" || { echo "fylo root $folder is not under $(atlas_mount_point)/slice-*, so its data is kept"; return 0; }
  rm -rf "$folder" && echo "fylo root $folder emptied for this run"
}

ensure_fylo_folder() {
  local folder="$1"
  [ -n "$folder" ] || { echo "no fylo:// YON_DATASOURCE_URL in .env"; return 0; }
  folder_storage_ready "$folder" || return 1
  emptied_fylo_folder "$folder" || return 1
  mkdir -p "$folder" && echo "fylo root $folder"
}

prepare_fylo_root() {
  local root="$1" app="$2"
  ensure_app_env "$root" "$app" || return 1
  ensure_fylo_folder "$(datasource_folder "$app")"
}

unconverted_list() {
  echo "$(repository_root)/tools/controllers/unconverted.list"
}

app_not_converted() {
  grep -qx "$(basename "$1")" "$(unconverted_list)"
}

error_number_check() {
  local root="$1"
  (cd "$root/tools/errornumbers" && go run . "$root")
}

controller_check() {
  local app="$1"
  if app_not_converted "$app"; then
    echo "$(basename "$app") is not converted yet (tools/controllers/unconverted.list); check skipped"
    return 0
  fi
  (cd "$(repository_root)/tools/controllers" && go run . "$app")
}

remove_app_build_output() {
  local app="$1"
  if [ -n "${MESAIL_KEEP_TACHYON:-}" ]; then return 0; fi
  if [ ! -d "$app/.tachyon" ]; then return 0; fi
  echo "=== $(basename "$app"): removing $(du -sh "$app/.tachyon" | cut -f1) of build output in $app/.tachyon"
  rm -rf "$app/.tachyon"
}

app_gates() {
  local ty="$1" app="$2" log="$3" status=0
  app_gate_steps "$ty" "$app" "$log" || status=1
  remove_app_build_output "$app" | tee -a "$log"
  return "$status"
}

app_gate_steps() {
  local ty="$1" app="$2" log="$3"
  local name status=0
  name="$(basename "$app")"
  if uses_fylo "$app"; then
    run_step "$name fylo root" "$log" prepare_fylo_root "$(repository_root)" "$app" || return 1
  fi
  if app_kind_has_server "$app"; then
    run_step "$name controllers" "$log" controller_check "$app" || status=1
  fi
  if ! app_kind_has_client "$app"; then
    run_step "$name settings" "$log" client_view_check "$app" || status=1
  fi
  run_step "$name migrate check" "$log" "$ty" migrate check "$app" || status=1
  run_step "$name test" "$log" "$ty" test "$app" --coverage || status=1
  if has_local_packages "$app"; then
    run_step "$name packages" "$log" local_package_checks "$app" "$(dirname "$log")/$name-packages.cover" || status=1
  fi
  if app_kind_has_client "$app"; then
    client_checks "$app" "$log" || status=1
  fi
  run_step "$name bundle" "$log" "$ty" bundle "$app" --target web || return 1
  if app_kind_has_server "$app"; then
    run_step "$name starts" "$log" app_starts "$ty" "$app" || status=1
  fi
  if has_responsive_test "$app"; then
    run_step "$name responsive" "$log" responsive_test "$app" || status=1
  fi
  return "$status"
}

app_kind_has_client() {
  [ -d "$1/client" ]
}

has_responsive_test() {
  [ "$(basename "$1")" = web ]
}

client_checks() {
  local app="$1" log="$2" root name status=0
  root="$(repository_root)"
  name="$(basename "$app")"
  run_step "$name duvay" "$log" "$root/scripts/vendor/duvay.sh" --check --app "$name" || status=1
  run_step "$name styles" "$log" sh -c "cd '$root/tools/styles' && go run . --check --app '$name'" || status=1
  run_step "$name views" "$log" client_view_check "$app" || status=1
  return "$status"
}

client_view_check() {
  local app="$1"
  (cd "$(repository_root)/tools/clientviews" && go run . "$app")
}

responsive_test() {
  local app="$1"
  (cd "$(repository_root)/tests/responsive" && MESAIL_DIST="$app/dist/web" go test -tags responsive -count=1 -v ./...)
}

start_wait_seconds() {
  echo "${MESAIL_START_TIMEOUT:-120}"
}

ready_address() {
  sed -n 's|^Tachyon production server ready at \(http://[^/ ]*\)/.*|\1|p' "$1" | head -1
}

address_when_ready() {
  local log="$1" pid="$2" waited=0 address=""
  while [ -z "$address" ] && [ "$waited" -lt "$(start_wait_seconds)" ] && kill -0 "$pid" 2> /dev/null; do
    sleep 1
    waited=$((waited + 1))
    address="$(ready_address "$log")"
  done
  echo "$address"
}

health_code() {
  curl -s -o /dev/null -w '%{http_code}' --max-time 10 "$1"
}

health_code_after_retries() {
  local url="$1" attempt code=""
  for attempt in 1 2 3 4 5 6 7 8 9 10; do
    code="$(health_code "$url")"
    [ "$code" = 200 ] && break
    sleep 1
  done
  echo "$code"
}

health_answers() {
  local address="$1" app="$2" url code
  [ -n "$address" ] || { echo "ty start did not report a ready address within $(start_wait_seconds) s"; return 1; }
  url="$address/api/v1/$(basename "$app")/health"
  code="$(health_code_after_retries "$url")"
  echo "GET $url -> $code"
  [ "$code" = 200 ]
}

stop_app() {
  kill "$1" 2> /dev/null
  wait "$1" 2> /dev/null
  trap - EXIT
  return 0
}

app_starts() {
  local ty="$1" app="$2" log pid code
  log="$(mktemp)"
  "$ty" start "$app" --port 0 > "$log" 2>&1 &
  pid=$!
  trap "kill $pid 2> /dev/null" EXIT
  health_answers "$(address_when_ready "$log" "$pid")" "$app"
  code=$?
  stop_app "$pid"
  cat "$log"
  rm -f "$log"
  return "$code"
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
  run_step "gofmt role files" "$folder/gofmt.log" role_files_formatted "$root" || status=1
  run_step "kit sync" "$folder/kit-sync.log" "$root/scripts/kit-sync.sh" --check || status=1
  run_step "go cache probe" "$folder/go-cache.log" "$root/scripts/go-cache_test.sh" || status=1
  run_step "fylo image attach" "$folder/fylo-image.log" "$root/scripts/fylo-image_test.sh" || status=1
  run_step "floci capture before retry" "$folder/floci-capture.log" "$root/scripts/floci/capture_test.sh" || status=1
  module_gates "$root" "$folder" || status=1
  return "$status"
}

gates_all_wanted() {
  [ "${MESAIL_GATES_ALL:-}" = 1 ] || has_flag --all "$@"
}

app_selection() {
  local only
  only="$(option_value --app "$@")"
  if [ -n "$only" ]; then echo "app:$only"; return; fi
  if gates_all_wanted "$@"; then echo all; return; fi
  echo changed
}

plan_base() {
  echo "${MESAIL_GATES_BASE:-origin/develop}"
}

changed_app_plan() {
  local root="$1" plan
  if plan="$(cd "$root/tools/gateplan" && go run . --root "$root" --base "$(plan_base)")" && [ -n "$plan" ]; then
    echo "$plan"
    return
  fi
  printf '*\ttools/gateplan failed, so every app runs\n'
}

app_plan() {
  local root="$1" selection="$2"
  case "$selection" in
    app:*) printf '%s\t--app\n' "${selection#app:}" ;;
    all) printf '*\t--all\n' ;;
    *) changed_app_plan "$root" ;;
  esac
}

plan_summary() {
  awk -F '\t' '{ name = $1; if (name == "*") name = "every app"; if (name == "-") name = "none"; printf "%s%s (%s)", (NR > 1 ? ", " : ""), name, $2 } END { print "" }'
}

planned_app() {
  local root="$1" name="$2"
  case "$name" in
    '*') selected_apps "$root" "" ;;
    -) ;;
    *) existing_app "$root/apps/$name" ;;
  esac
}

planned_apps() {
  local root="$1" plan="$2" name
  while IFS="$(printf '\t')" read -r name _; do
    planned_app "$root" "$name" || return 1
  done <<< "$plan"
}

show_app_plan() {
  local root="$1" selection="$2"
  echo "=== apps chosen: $(app_plan "$root" "$selection" | plan_summary)"
}

all_app_gates() {
  local root="$1" ty="$2" folder="$3" selection="$4" plan apps app status=0
  plan="$(app_plan "$root" "$selection")"
  echo "=== apps chosen: $(echo "$plan" | plan_summary)"
  apps="$(planned_apps "$root" "$plan")" || return 1
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
      --app | --all | --plan | --shared-only | --apps-only | --list-apps) ;;
      *) [ "$previous" = --app ] || { echo "gates: unknown option $argument" >&2; return 0; } ;;
    esac
    previous="$argument"
  done
  return 1
}

usage() {
  echo "usage: scripts/gates.sh [--app <name> | --all] [--shared-only | --apps-only] | --plan | --list-apps" >&2
  echo "  the app steps run only for the apps the change touches (tools/gateplan, SLICES K15)," >&2
  echo "  found from the files that differ from origin/develop (committed since the branch point," >&2
  echo "  uncommitted, or new; MESAIL_GATES_BASE names another base), printed first as" >&2
  echo "  '=== apps chosen: <app> (<reason>), ...'. An app is chosen for its own files (apps/<app>/**)" >&2
  echo "  and for a changed kit/<package> that its server/packages/kit.list names; web also for" >&2
  echo "  tests/responsive. Every app runs when scripts/gates.sh, go-cache.sh, kit-sync.sh, scripts/ci," >&2
  echo "  scripts/vendor, tools, .github, any go.mod or go.sum, or a path the plan does not know" >&2
  echo "  changed, when no file differs, and when the changed files cannot be listed. Docs and" >&2
  echo "  shared code (edge, deploy, tests, kit files no app copies) choose no app." >&2
  echo "  --all          every app (also MESAIL_GATES_ALL=1); the nightly full run uses it" >&2
  echo "  --app <name>   exactly that app" >&2
  echo "  --plan         print the apps the run would choose, and stop" >&2
  echo "  --list-apps    print every app's name (the CI matrix)" >&2
  echo "  --shared-only  versions, doctor, gofmt of role files, kit sync, and in every Go module" >&2
  echo "                 outside apps/ (found by its go.mod): gofmt, go vet (with the module's" >&2
  echo "                 build tags), go test; modules outside tests/ and tools/ also pass CRAP <= 15" >&2
  echo "  --apps-only    versions and the per-app steps; --app <name> limits them to one app" >&2
  echo "                 (a backend app's steps start with tools/controllers, ADR 0009, unless the app" >&2
  echo "                 is still in tools/controllers/unconverted.list)" >&2
  echo "                 (a Tac app's steps include tools/clientviews, SLICES rule 19: the manifest keeps" >&2
  echo "                 tac.dependencies.check on, and no tac.html view holds a ? : expression or" >&2
  echo "                 builds a value with +; every app's steps check that server/application.json keeps" >&2
  echo "                 dependencies.check on and .env and .env.example never turn TAC_DEPENDENCIES_CHECK off)" >&2
  echo "  FYLO roots go under ~/Developer/AtlasFylo/mesail/slice-<id>: <id> is MESAIL_SLICE when set, else" >&2
  echo "  the worktree folder's name without a slice- prefix (e12-kit-errors -> slice-e12-kit-errors);" >&2
  echo "  MESAIL_FYLO_ROOT overrides the whole folder. A worktree that set MESAIL_SLICE before keeps" >&2
  echo "  its folder only while it still sets it; otherwise the old slice-<id> folder is left behind," >&2
  echo "  and its owner removes it after the merge (native runs keep \$TMPDIR/mesail-native/<owner>)" >&2
  echo "  each app's build output (apps/<app>/.tachyon) is removed when its steps end, pass or fail;" >&2
  echo "  MESAIL_KEEP_TACHYON=1 keeps it (for debugging)" >&2
  echo "  every run also checks docs/mesail/ERRORS.md against the error numbers in code" >&2
  echo "  (tools/errornumbers, ADR 0009; apps in tools/controllers/unconverted.list are skipped)" >&2
  echo "  MESAIL_GOCACHE  the Go build cache (default on macOS: ~/Library/Caches/go-build-mesail," >&2
  echo "                  a local disk (on this Mac a link to HANGAR), not the ANVIL NFS link of ~/Library/Caches/go-build)" >&2
  echo "                  Go runs with -trimpath, so worktrees share cache entries; an unusable cache" >&2
  echo "                  folder, or one that does not answer within MESAIL_GO_CACHE_PROBE_SECONDS (10)," >&2
  echo "                  falls back to \$TMPDIR/go-build-mesail-fallback; under" >&2
  echo "                  MESAIL_GOCACHE_MIN_FREE_GB (35) GiB free, entries unused for over 1 hour" >&2
  echo "                  are removed before the run; over MESAIL_GOCACHE_MAX_GB (10) GiB, the same," >&2
  echo "                  then entries unused for over 10 minutes if it is still over (never a full" >&2
  echo "                  clean: other sessions share the cache)" >&2
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
  local root="$1" folder="$2" selection="$3"
  shift 3
  local ty fylo status=0
  ty="$(tachyon_binary)"
  fylo="$(fylo_binary)"
  run_step "versions" "$folder/versions.log" check_versions "$ty" "$fylo" || return 1
  use_pinned_tools "$ty" "$fylo"
  run_step "error numbers" "$folder/error-numbers.log" error_number_check "$root" || status=1
  if wanted --apps-only "$@"; then shared_gates "$root" "$ty" "$folder" || status=1; fi
  if wanted --shared-only "$@"; then all_app_gates "$root" "$ty" "$folder" "$selection" || status=1; fi
  return "$status"
}

main() {
  local root folder selection status
  mesail_use_local_go_cache
  if has_unknown_option "$@"; then usage; return; fi
  root="$(repository_root)"
  if has_flag --list-apps "$@"; then list_apps "$root"; return; fi
  selection="$(app_selection "$@")"
  if has_flag --plan "$@"; then show_app_plan "$root" "$selection"; return; fi
  folder="$(log_folder)"
  echo "gates: logs in $folder"
  run_gates "$root" "$folder" "$selection" "$@"
  status=$?
  echo "=== gates rc=$status"
  return "$status"
}

main "$@"
