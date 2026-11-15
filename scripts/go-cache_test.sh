#!/usr/bin/env bash
# shellcheck shell=bash
set -uo pipefail
# shellcheck source-path=SCRIPTDIR
source "$(cd "$(dirname "$0")" && pwd)/go-cache.sh"

go_cache_test_fail() {
  echo "FAIL $1: $2" >&2
  exit 1
}

go_cache_test_expect_equal() {
  local name="$1" expected="$2" actual="$3"
  if [ "$expected" != "$actual" ]; then go_cache_test_fail "$name" "expected '$expected', got '$actual'"; fi
  echo "ok   $name"
}

go_cache_test_expect_contains() {
  local name="$1" text="$2" file="$3"
  if ! grep -q -F -- "$text" "$file"; then go_cache_test_fail "$name" "'$text' not in $(cat "$file")"; fi
  echo "ok   $name"
}

go_cache_test_expect_empty() {
  local name="$1" file="$2"
  if [ -s "$file" ]; then go_cache_test_fail "$name" "expected no output, got $(cat "$file")"; fi
  echo "ok   $name"
}

go_cache_test_expect_at_most() {
  local name="$1" limit="$2" actual="$3"
  if [ "$actual" -gt "$limit" ]; then go_cache_test_fail "$name" "took $actual s, limit $limit s"; fi
  echo "ok   $name"
}

go_cache_test_usable_folder() {
  local work="$1"
  ( export TMPDIR="$work/tmp" MESAIL_GOCACHE="$work/cache"
    mesail_use_local_go_cache 2> "$work/usable.err"
    echo "$GOCACHE" > "$work/usable.out" )
  go_cache_test_expect_equal "a usable folder is chosen" "$work/cache" "$(cat "$work/usable.out")"
  go_cache_test_expect_empty "a usable folder prints no warning" "$work/usable.err"
}

go_cache_test_unusable_folder() {
  local work="$1"
  ( export TMPDIR="$work/tmp" MESAIL_GOCACHE="/dev/null/go-cache"
    mesail_use_local_go_cache 2> "$work/unusable.err"
    echo "$GOCACHE" > "$work/unusable.out" )
  go_cache_test_expect_equal "an unusable folder falls back" "$work/tmp/go-build-mesail-fallback" "$(cat "$work/unusable.out")"
  go_cache_test_expect_contains "an unusable folder is named" "is not usable or did not answer" "$work/unusable.err"
}

go_cache_test_stuck_folder() {
  local work="$1" started finished
  started="$(date +%s)"
  ( export TMPDIR="$work/tmp" MESAIL_GOCACHE="$work/stuck" MESAIL_GO_CACHE_PROBE_SECONDS=1
    mesail_go_cache_usable() { sleep 5; }
    mesail_use_local_go_cache 2> "$work/stuck.err"
    echo "$GOCACHE" > "$work/stuck.out" )
  finished="$(date +%s)"
  go_cache_test_expect_equal "a stuck folder falls back" "$work/tmp/go-build-mesail-fallback" "$(cat "$work/stuck.out")"
  go_cache_test_expect_contains "a stuck folder is reported" "did not answer within 1 s" "$work/stuck.err"
  go_cache_test_expect_at_most "a stuck folder does not hold the run" 3 "$((finished - started))"
}

go_cache_test_main() {
  local work
  export MESAIL_GOCACHE_MIN_FREE_GB=0
  work="$(mktemp -d)"
  mkdir -p "$work/tmp"
  go_cache_test_usable_folder "$work"
  go_cache_test_unusable_folder "$work"
  go_cache_test_stuck_folder "$work"
  rm -rf "$work"
  echo "go-cache: all checks passed"
}

go_cache_test_main
