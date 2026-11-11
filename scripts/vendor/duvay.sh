#!/usr/bin/env bash
set -euo pipefail

duvay_repository() { echo "https://github.com/d31ma/DUVAY"; }
duvay_release() { echo "develop"; }
duvay_version() { echo "26.46.04-develop.d68028d"; }
duvay_commit() { echo "d68028d35c013948b8bc880d813542f8e3d79917"; }
package_name() { echo "@d31ma/duvay"; }
package_license() { echo "MIT"; }
script_name() { echo "duvay-mesail.js"; }
style_name() { echo "duvay-mesail.css"; }
bundle_flags() { echo "--target=browser --format=esm --minify"; }

fail() {
  echo "duvay: $1" >&2
  exit 1
}

usage() {
  echo "usage: scripts/vendor/duvay.sh [--app <app>] [--check]" >&2
  exit 2
}

repository_root() {
  cd "$(dirname "$0")/../.." && pwd
}

vendor_folder() {
  cd "$(dirname "$0")" && pwd
}

cache_folder() {
  echo "${XDG_CACHE_HOME:-$HOME/Library/Caches}/mesail-vendor/duvay-$(duvay_commit)"
}

run_step() {
  local step="$1"
  shift
  echo "=== $step $(date +%H:%M:%S)"
  local status=0
  "$@" || status=$?
  echo "=== $step rc=$status"
  return "$status"
}

read_names() {
  local list_file="$1" name
  while IFS= read -r name || [ -n "$name" ]; do
    [ -n "$name" ] || continue
    [[ "$name" =~ ^[a-z0-9-]+$ ]] || fail "bad name '$name' in $list_file"
    echo "$name"
  done < "$list_file"
}

source_is_pinned() {
  local source="$1"
  [ -d "$source/.git" ] || return 1
  [ "$(git -C "$source" rev-parse HEAD 2>/dev/null)" = "$(duvay_commit)" ] || return 1
  [ -z "$(git -C "$source" status --porcelain --untracked-files=all)" ]
}

download_source() {
  local source="$1"
  case "$source" in
    */mesail-vendor/duvay-"$(duvay_commit)") rm -rf "$source" ;;
    *) fail "refusing to replace unexpected cache folder $source" ;;
  esac
  mkdir -p "$source" &&
  git -C "$source" init --quiet &&
  git -C "$source" remote add origin "$(duvay_repository)" &&
  git -C "$source" -c core.hooksPath=/dev/null fetch --depth 1 --no-tags origin "$(duvay_commit)" &&
  git -C "$source" -c core.hooksPath=/dev/null -c advice.detachedHead=false checkout --detach FETCH_HEAD
}

fetch_source() {
  local source="$1"
  if source_is_pinned "$source"; then
    echo "using cached DuVay $(duvay_commit) in $source"
    return 0
  fi
  download_source "$source" || return 1
  source_is_pinned "$source" || fail "DuVay checkout in $source is not commit $(duvay_commit)"
  echo "fetched DuVay $(duvay_commit) into $source"
}

check_components() {
  local source="$1" components_file="$2" name
  for name in $(read_names "$components_file"); do
    [ -f "$source/src/components/$name.js" ] || fail "DuVay has no component src/components/$name.js"
  done
}

icon_set_path() {
  echo "docs/mesail/ux/mockups/build/mesail-icons.js"
}

check_icon_set() {
  local root="$1"
  [ -f "$root/$(icon_set_path)" ] && return 0
  fail "Mesail's icon set $(icon_set_path) is missing; it comes from mesail-ux's mockups build on develop, so rebase onto develop after that merge and run again"
}

write_script_entry() {
  local components_file="$1" name
  echo "import './src/icons.js';"
  echo "import './mesail-icons.js';"
  echo "import './src/components/base.js';"
  for name in $(read_names "$components_file"); do
    echo "import './src/components/$name.js';"
  done
}

bundle_script() {
  local source="$1" work="$2" components_file="$3" flags
  read -ra flags <<< "$(bundle_flags)"
  check_components "$source" "$components_file"
  cp -R "$source/src" "$work/src" || return 1
  cp "$(repository_root)/$(icon_set_path)" "$work/mesail-icons.js" || return 1
  write_script_entry "$components_file" > "$work/entry.js" || return 1
  (cd "$work" && bun build entry.js "${flags[@]}" --metafile=meta.json --outfile="out/$(script_name)")
}

style_order() {
  local source="$1" index
  for index in $(grep -oE '"\./[a-z-]+\.css"' "$source/src/duvay.css" | tr -d '"'); do
    grep -oE 'components/[a-z0-9-]+\.css' "$source/src/$index" || true
  done | sed -e 's#^components/##' -e 's#\.css$##'
}

core_styles() {
  local source="$1"
  grep -oE '"\./[a-z-]+\.css"' "$source/src/core.css" | tr -d '"' | sed -e 's#^\./##' -e 's#\.css$##'
}

ordered_styles() {
  local source="$1" styles_file="$2" order name
  order="$(style_order "$source")"
  for name in $(read_names "$styles_file"); do
    grep -qx "$name" <<< "$order" || fail "DuVay has no indexed style src/components/$name.css"
  done
  grep -x -F -f <(read_names "$styles_file") <<< "$order"
}

assemble_styles() {
  local source="$1" styles_file="$2" name
  echo "@layer duvay {"
  for name in $(core_styles "$source"); do
    perl "$(vendor_folder)/duvay-utility-filter.pl" < "$source/src/$name.css" || return 1
  done
  for name in $(ordered_styles "$source" "$styles_file"); do
    perl -0pe 's{/\*.*?\*/}{}gs; s{\@import[^;]*;}{}g' "$source/src/components/$name.css" || return 1
  done
  echo "}"
}

bundle_styles() {
  local source="$1" work="$2" styles_file="$3"
  assemble_styles "$source" "$styles_file" > "$work/styles.css" || return 1
  (cd "$work" && bun build styles.css --minify --outfile="out/$(style_name)")
}

copy_license() {
  local source="$1" work="$2"
  cp "$source/LICENSE" "$work/out/LICENSE"
}

sha256_of() {
  shasum -a 256 "$1" | cut -d' ' -f1
}

style_inputs() {
  local source="$1" styles_file="$2" name
  for name in $(core_styles "$source"); do
    echo "src/$name.css"
  done
  for name in $(ordered_styles "$source" "$styles_file"); do
    echo "src/components/$name.css"
  done
}

input_hashes() {
  local source="$1" work="$2" styles_file="$3" input
  {
    jq -r '.inputs | keys[] | select(startswith("src/"))' "$work/meta.json"
    style_inputs "$source" "$styles_file"
    echo "LICENSE"
  } | LC_ALL=C sort -u | while IFS= read -r input; do
    echo "$input $(sha256_of "$source/$input")"
  done
}

output_hashes() {
  local work="$1" file
  for file in "$(script_name)" "$(style_name)" LICENSE; do
    echo "$file $(sha256_of "$work/out/$file")"
  done
}

lines_to_object() {
  jq -R -s 'split("\n") | map(select(length > 0) | split(" ")) | map({(.[0]): .[1]}) | add'
}

write_provenance() {
  local source="$1" work="$2" components_file="$3" styles_file="$4" built_at="$5"
  jq -n \
    --arg package "$(package_name)" \
    --arg version "$(duvay_version)" \
    --arg license "$(package_license)" \
    --arg repository "$(duvay_repository)" \
    --arg release "$(duvay_release)" \
    --arg commit "$(duvay_commit)" \
    --arg bundler "bun $(bun --version)" \
    --arg flags "$(bundle_flags)" \
    --arg built_at "$built_at" \
    --arg icon_set "$(icon_set_path)" \
    --arg icon_set_sha "$(sha256_of "$(repository_root)/$(icon_set_path)")" \
    --argjson components "$(read_names "$components_file" | jq -R . | jq -s .)" \
    --argjson styles "$(ordered_styles "$source" "$styles_file" | jq -R . | jq -s .)" \
    --argjson inputs "$(input_hashes "$source" "$work" "$styles_file" | lines_to_object)" \
    --argjson outputs "$(output_hashes "$work" | lines_to_object)" \
    '{package: $package, version: $version, license: $license,
      source: {repository: $repository, release: $release, commit: $commit},
      components: $components, styles: $styles,
      iconSet: {file: $icon_set, sha256: $icon_set_sha},
      build: {bundler: $bundler, flags: ($flags | split(" ")), cssLayer: "duvay", builtAt: $built_at},
      inputs: $inputs, outputs: $outputs}' > "$work/out/PROVENANCE.json"
}

work_folder() {
  echo "${TMPDIR:-/tmp}/duvay-vendor.$$"
}

current_build_date() {
  date -u +%Y-%m-%dT%H:%M:%SZ
}

previous_build_date() {
  local output="$1"
  [ -f "$output/PROVENANCE.json" ] || return 0
  jq -r '.build.builtAt // empty' "$output/PROVENANCE.json"
}

build_into() {
  local source="$1" work="$2" components_file="$3" styles_file="$4" built_at="$5"
  mkdir -p "$work/out"
  read_names "$components_file" > /dev/null
  ordered_styles "$source" "$styles_file" > /dev/null
  run_step bundle-script bundle_script "$source" "$work" "$components_file"
  run_step bundle-styles bundle_styles "$source" "$work" "$styles_file"
  run_step license copy_license "$source" "$work"
  run_step provenance write_provenance "$source" "$work" "$components_file" "$styles_file" "$built_at"
}

same_output() {
  local work="$1" output="$2"
  diff -r "$work/out" "$output" > /dev/null 2>&1
}

install_output() {
  local source="$1" work="$2" output="$3" components_file="$4" styles_file="$5"
  write_provenance "$source" "$work" "$components_file" "$styles_file" "$(current_build_date)" || return 1
  case "$output" in
    */client/shared/vendor/duvay) rm -rf "$output" ;;
    *) fail "refusing to replace unexpected output folder $output" ;;
  esac
  mkdir -p "$output" || return 1
  cp "$work/out/"* "$output/" || return 1
  echo "wrote $output"
}

report_difference() {
  local work="$1" output="$2"
  diff -rq "$work/out" "$output" >&2 || true
  fail "vendored DuVay in $output is stale; run scripts/vendor/duvay.sh"
}

manifest_entries() {
  local output="$1" script_path style_path
  script_path="/shared/vendor/duvay/$(script_name)"
  style_path="/shared/vendor/duvay/$(style_name)"
  jq -n \
    --arg script "$script_path" \
    --arg style "$style_path" \
    --arg package "$(package_name)" \
    --arg version "$(duvay_version)" \
    --arg license "$(package_license)" \
    --arg script_sha "$(sha256_of "$output/$(script_name)")" \
    --arg style_sha "$(sha256_of "$output/$(style_name)")" \
    '{scripts: [$script],
      scriptSources: {($script): {package: $package, version: $version, license: $license, sha256: $script_sha}},
      styleSources: {($style): {kind: "vendor", package: $package, version: $version, license: $license, sha256: $style_sha}}}'
}

parse_arguments() {
  local app="web" mode="build"
  while [ "$#" -gt 0 ]; do
    case "$1" in
      --app) [ "$#" -ge 2 ] || usage; app="$2"; shift 2 ;;
      --check) mode="check"; shift ;;
      *) usage ;;
    esac
  done
  [[ "$app" =~ ^[a-z0-9-]+$ ]] || fail "bad app name '$app'"
  echo "$app $mode"
}

main() {
  local arguments app mode root output source work built_at
  parse_arguments "$@" > /dev/null
  arguments="$(parse_arguments "$@")"
  app="${arguments% *}"
  mode="${arguments#* }"
  root="$(repository_root)"
  [ -d "$root/apps/$app/client" ] || fail "no app folder apps/$app/client"
  output="$root/apps/$app/client/shared/vendor/duvay"
  check_icon_set "$root"
  source="$(cache_folder)"
  work="$(work_folder)"
  trap 'rm -rf "$(work_folder)"' EXIT
  mkdir "$work"
  run_step fetch fetch_source "$source"
  built_at="$(previous_build_date "$output")"
  build_into "$source" "$work" "$(vendor_folder)/duvay-components.txt" "$(vendor_folder)/duvay-styles.txt" "${built_at:-$(current_build_date)}"
  if same_output "$work" "$output"; then
    echo "vendored DuVay in apps/$app/client/shared/vendor/duvay is up to date"
  elif [ "$mode" = check ]; then
    report_difference "$work" "$output"
  else
    run_step install install_output "$source" "$work" "$output" "$(vendor_folder)/duvay-components.txt" "$(vendor_folder)/duvay-styles.txt"
  fi
  echo "=== manifest entries for apps/$app/client/manifest.json (tac)"
  manifest_entries "$output"
  echo "=== first lines of apps/$app/client/shared/styles/app.css"
  echo "@layer duvay, theme, base, components, utilities;"
  echo "@import \"../vendor/duvay/$(style_name)\" layer(duvay);"
}

main "$@"
