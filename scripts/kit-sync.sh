#!/usr/bin/env bash
set -euo pipefail

repository_root() {
  cd "$(dirname "$0")/.." && pwd
}

write_go_mod() {
  local packages_dir="$1"
  printf 'module mesail.dev/packages\n\ngo 1.27\n' > "$packages_dir/go.mod"
}

copy_package() {
  local root="$1" packages_dir="$2" package_name="$3"
  rm -rf "${packages_dir:?}/$package_name"
  mkdir -p "$packages_dir/$package_name"
  find "$root/kit/$package_name" -maxdepth 1 -name '*.go' ! -name '*_test.go' -exec cp {} "$packages_dir/$package_name/" \;
}

check_package() {
  local root="$1" packages_dir="$2" package_name="$3"
  diff -r -x '*_test.go' "$root/kit/$package_name" "$packages_dir/$package_name" > /dev/null || {
    echo "kit-sync: $packages_dir/$package_name differs from kit/$package_name; run scripts/kit-sync.sh" >&2
    return 1
  }
}

sync_app() {
  local root="$1" list_file="$2" mode="$3"
  local packages_dir package_name status=0
  packages_dir="$(dirname "$list_file")"
  [ "$mode" = check ] || write_go_mod "$packages_dir"
  while IFS= read -r package_name; do
    [ -n "$package_name" ] || continue
    if [ "$mode" = check ]; then
      check_package "$root" "$packages_dir" "$package_name" || status=1
    else
      copy_package "$root" "$packages_dir" "$package_name"
      echo "kit-sync: kit/$package_name -> ${packages_dir#"$root"/}/$package_name"
    fi
  done < "$list_file"
  return "$status"
}

main() {
  local root mode=write list_file status=0
  root="$(repository_root)"
  [ "${1:-}" = --check ] && mode=check
  for list_file in "$root"/apps/*/server/packages/kit.list; do
    [ -f "$list_file" ] || continue
    sync_app "$root" "$list_file" "$mode" || status=1
  done
  [ "$status" = 0 ] && echo "kit-sync: $mode ok"
  return "$status"
}

main "$@"
