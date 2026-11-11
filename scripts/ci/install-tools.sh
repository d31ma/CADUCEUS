#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "$0")" && pwd)/tool-lock.sh"

download_folder() {
  local destination="$1"
  echo "${MESAIL_CI_CACHE:-$destination/downloads}"
}

fetch_verified() {
  local url="$1" sha="$2" folder="$3" partial
  partial="$folder/$sha.$$.partial"
  echo "=== download $url $(date +%H:%M:%S)" >&2
  curl -fsSL --retry 3 "$url" -o "$partial" || return 1
  tool_lock_verify "$partial" "$sha" || return 1
  mv "$partial" "$folder/$sha"
  echo "=== verified $sha" >&2
}

cached_download() {
  local url="$1" sha="$2" folder="$3"
  mkdir -p "$folder"
  if [ -f "$folder/$sha" ] && tool_lock_verify "$folder/$sha" "$sha"; then
    echo "=== reuse $url" >&2
    echo "$folder/$sha"
    return
  fi
  fetch_verified "$url" "$sha" "$folder" || return 1
  echo "$folder/$sha"
}

unpack() {
  local tool="$1" archive="$2" destination="$3" binary_folder="$4"
  if [ "$binary_folder" = - ]; then
    install -m 755 "$archive" "$destination/bin/$tool"
    echo "$destination/bin"
    return
  fi
  rm -rf "${destination:?}/$tool"
  mkdir -p "$destination/$tool"
  tar -xzf "$archive" -C "$destination/$tool" || return 1
  echo "$destination/$tool/$binary_folder"
}

install_tool() {
  local destination="$1" tool="$2" platform="$3" entry archive
  entry="$(tool_lock_entry "$tool" "$platform")" || return 1
  read -r _ version _ url sha binary_folder <<< "$entry"
  echo "=== install $tool $version for $platform" >&2
  archive="$(cached_download "$url" "$sha" "$(download_folder "$destination")")" || return 1
  unpack "$tool" "$archive" "$destination" "$binary_folder"
}

requested_tools() {
  local platform="$1"
  shift
  [ "$#" -gt 0 ] && { printf '%s\n' "$@"; return; }
  tool_lock_tools "$platform"
}

publish_folder() {
  local folder="$1"
  echo "$folder" >> "${GITHUB_PATH:-/dev/null}"
}

joined_path() {
  awk 'NF && !seen[$0]++ { lines[++count] = $0 } END { for (index_ = count; index_ > 0; index_--) print lines[index_] }' | paste -sd: -
}

main() {
  local destination="${1:?usage: install-tools.sh <folder> [tool ...]}"
  shift
  local platform tool folder folders=""
  platform="$(tool_lock_platform)"
  mkdir -p "$destination/bin"
  destination="$(cd "$destination" && pwd)"
  for tool in $(requested_tools "$platform" "$@"); do
    folder="$(install_tool "$destination" "$tool" "$platform")" || exit 1
    publish_folder "$folder"
    folders="$folders$folder"$'\n'
  done
  echo "export PATH=\"$(printf '%s' "$folders" | joined_path):\$PATH\""
}

main "$@"
