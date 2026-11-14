#!/usr/bin/env bash
set -euo pipefail
# shellcheck source-path=SCRIPTDIR
source "$(cd "$(dirname "$0")" && pwd)/go-cache.sh"

repository_root() {
  cd "$(dirname "$0")/.." && pwd
}

copy_package() {
  local root="$1" packages_dir="$2" package_name="$3"
  rm -rf "${packages_dir:?}/$package_name"
  mkdir -p "$packages_dir/$package_name"
  find "$root/kit/$package_name" -maxdepth 1 -name '*.go' ! -name '*_test.go' -exec cp {} "$packages_dir/$package_name/" \;
}

copy_module_files() {
  local root="$1" packages_dir="$2"
  cp "$root/kit/go.mod" "$packages_dir/go.mod"
  rm -f "$packages_dir/go.sum"
  if [ -f "$root/kit/go.sum" ]; then
    cp "$root/kit/go.sum" "$packages_dir/go.sum"
  fi
}

has_outside_modules() {
  grep -q '^require' "$1/go.mod"
}

vendor_modules() {
  local packages_dir="$1"
  rm -rf "$packages_dir/vendor"
  if ! has_outside_modules "$packages_dir"; then
    rm -f "$packages_dir/go.sum"
    return 0
  fi
  (cd "$packages_dir" && go mod tidy && go mod vendor)
  if ! has_outside_modules "$packages_dir"; then
    rm -rf "$packages_dir/go.sum" "$packages_dir/vendor"
  fi
}

write_app() {
  local root="$1" packages_dir="$2" package_name
  copy_module_files "$root" "$packages_dir"
  while IFS= read -r package_name; do
    [ -n "$package_name" ] || continue
    copy_package "$root" "$packages_dir" "$package_name"
  done < "$packages_dir/kit.list"
  vendor_modules "$packages_dir"
}

check_app() {
  local root="$1" packages_dir="$2" scratch
  scratch="$(mktemp -d)"
  cp -R "$packages_dir/." "$scratch/"
  write_app "$root" "$scratch" > /dev/null
  if ! diff -r "$scratch" "$packages_dir" > /dev/null; then
    echo "kit-sync: ${packages_dir#"$root"/} differs from kit/; run scripts/kit-sync.sh" >&2
    diff -r -q "$scratch" "$packages_dir" >&2 || true
    rm -rf "$scratch"
    return 1
  fi
  rm -rf "$scratch"
}

sync_app() {
  local root="$1" packages_dir="$2" mode="$3"
  if [ "$mode" = check ]; then
    check_app "$root" "$packages_dir"
    return
  fi
  write_app "$root" "$packages_dir"
  echo "kit-sync: $(tr '\n' ' ' < "$packages_dir/kit.list")-> ${packages_dir#"$root"/}"
}

kit_vendoring_modules() {
  local root="$1"
  find "$root" \( -name node_modules -o -name .git -o -name .tachyon -o -name dist \) -prune \
    -o -type d -path '*/vendor/mesail.dev/packages' -print | sed 's|/vendor/mesail\.dev/packages$||' | sort
}

vendor_kit() {
  local module="$1" output="$2"
  (cd "$module" && GOFLAGS="${GOFLAGS:+$GOFLAGS }-mod=mod" go mod vendor -o "$output")
}

vendored_copy_matches() {
  local module="$1" scratch="$2"
  vendor_kit "$module" "$scratch/vendor" && diff -r "$scratch/vendor" "$module/vendor" > /dev/null
}

check_vendored_module() {
  local root="$1" module="$2" scratch
  scratch="$(mktemp -d)"
  if vendored_copy_matches "$module" "$scratch"; then
    rm -rf "$scratch"
    return 0
  fi
  echo "kit-sync: ${module#"$root"/}/vendor is not what go mod vendor makes from kit/; run scripts/kit-sync.sh" >&2
  diff -r -q "$scratch/vendor" "$module/vendor" >&2 || true
  rm -rf "$scratch"
  return 1
}

write_vendored_module() {
  local root="$1" module="$2"
  vendor_kit "$module" vendor
  echo "kit-sync: go mod vendor -> ${module#"$root"/}/vendor"
}

sync_vendored_module() {
  local root="$1" module="$2" mode="$3"
  if [ "$mode" = check ]; then
    check_vendored_module "$root" "$module"
    return
  fi
  write_vendored_module "$root" "$module"
}

sync_apps() {
  local root="$1" mode="$2" list_file status=0
  for list_file in "$root"/apps/*/server/packages/kit.list; do
    [ -f "$list_file" ] || continue
    sync_app "$root" "$(dirname "$list_file")" "$mode" || status=1
  done
  return "$status"
}

sync_vendored_modules() {
  local root="$1" mode="$2" module status=0
  while IFS= read -r module; do
    sync_vendored_module "$root" "$module" "$mode" || status=1
  done < <(kit_vendoring_modules "$root")
  return "$status"
}

main() {
  local root mode=write status=0
  mesail_use_local_go_cache
  root="$(repository_root)"
  [ "${1:-}" = --check ] && mode=check
  sync_apps "$root" "$mode" || status=1
  sync_vendored_modules "$root" "$mode" || status=1
  [ "$status" = 0 ] && echo "kit-sync: $mode ok"
  return "$status"
}

main "$@"
