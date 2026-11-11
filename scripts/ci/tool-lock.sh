#!/usr/bin/env bash

tool_lock_file() {
  echo "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/tools.lock"
}

tool_lock_system() {
  local system
  system="$(uname -s | tr '[:upper:]' '[:lower:]')"
  [ "$system" = darwin ] && { echo macos; return; }
  echo "$system"
}

tool_lock_machine() {
  case "$(uname -m)" in
    x86_64 | amd64) echo x64 ;;
    aarch64 | arm64) echo arm64 ;;
    *) uname -m ;;
  esac
}

tool_lock_platform() {
  echo "$(tool_lock_system)-$(tool_lock_machine)"
}

tool_lock_entry() {
  local tool="$1" platform="$2" entry
  entry="$(awk -v tool="$tool" -v platform="$platform" '$1 == tool && $3 == platform' "$(tool_lock_file)")"
  [ -n "$entry" ] || { echo "tools.lock has no $tool for $platform" >&2; return 1; }
  echo "$entry"
}

tool_lock_field() {
  local tool="$1" platform="$2" column="$3" entry
  entry="$(tool_lock_entry "$tool" "$platform")" || return 1
  echo "$entry" | awk -v column="$column" '{ print $column }'
}

tool_lock_version() {
  awk -v tool="$1" '$1 == tool { print $2; exit }' "$(tool_lock_file)"
}

tool_lock_tools() {
  awk -v platform="$1" '$3 == platform && !seen[$1]++ { print $1 }' "$(tool_lock_file)"
}

tool_lock_sha256() {
  if command -v sha256sum > /dev/null; then
    sha256sum "$1" | awk '{ print $1 }'
    return
  fi
  shasum -a 256 "$1" | awk '{ print $1 }'
}

tool_lock_verify() {
  local file="$1" expected="$2" actual
  actual="$(tool_lock_sha256 "$file")"
  [ "$actual" = "$expected" ] || { echo "checksum mismatch for $file: expected $expected, found $actual" >&2; return 1; }
}

tool_lock_verify_binary() {
  local tool="$1" binary="$2" expected
  expected="$(tool_lock_field "$tool" "$(tool_lock_platform)" 5)" || return 1
  tool_lock_verify "$binary" "$expected"
}
