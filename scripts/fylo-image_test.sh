#!/usr/bin/env bash
# shellcheck shell=bash
set -uo pipefail
# shellcheck source-path=SCRIPTDIR
source "$(cd "$(dirname "$0")" && pwd)/fylo-image.sh"

fylo_image_test_fail() {
  echo "FAIL $1: $2" >&2
  exit 1
}

fylo_image_test_expect_status() {
  local name="$1" expected="$2" actual="$3"
  if [ "$expected" != "$actual" ]; then fylo_image_test_fail "$name" "expected status $expected, got $actual"; fi
  echo "ok   $name"
}

fylo_image_test_expect_contains() {
  local name="$1" text="$2" file="$3"
  if ! grep -q -F -- "$text" "$file"; then fylo_image_test_fail "$name" "'$text' not in $(cat "$file")"; fi
  echo "ok   $name"
}

fylo_image_test_expect_at_most() {
  local name="$1" limit="$2" actual="$3"
  if [ "$actual" -gt "$limit" ]; then fylo_image_test_fail "$name" "took $actual s, limit $limit s"; fi
  echo "ok   $name"
}

fylo_image_test_expect_no_lock() {
  local name="$1"
  if [ -e "$(mesail_fylo_image_lock)" ]; then fylo_image_test_fail "$name" "the lock $(mesail_fylo_image_lock) was left behind"; fi
  echo "ok   $name"
}

fylo_image_test_make_image() {
  hdiutil create -quiet -size 20m -type SPARSEBUNDLE -fs APFS -volname mesail-k18-test "$1"
}

fylo_image_test_detach() {
  if mesail_fylo_image_mounted; then hdiutil detach -quiet "$(mesail_fylo_image_mount_point)"; fi
}

fylo_image_test_attaches_when_missing() {
  local work="$1" status
  mesail_fylo_image_ready 2> "$work/attach.err"
  status=$?
  fylo_image_test_expect_status "a missing mount is attached" 0 "$status"
  fylo_image_test_expect_status "the image is mounted after the attach" 0 "$(mesail_fylo_image_mounted; echo $?)"
  fylo_image_test_expect_contains "the attach is reported" "=== fylo image: attaching" "$work/attach.err"
  fylo_image_test_expect_no_lock "the lock is released after the attach"
}

fylo_image_test_keeps_a_mounted_image() {
  local work="$1" status
  mesail_fylo_image_ready 2> "$work/mounted.err"
  status=$?
  fylo_image_test_expect_status "a mounted image is kept" 0 "$status"
  fylo_image_test_expect_status "a mounted image prints nothing" 1 "$([ -s "$work/mounted.err" ]; echo $?)"
}

fylo_image_test_two_gates_at_once() {
  local work="$1" first second
  fylo_image_test_detach
  mesail_fylo_image_ready 2> "$work/first.err" &
  first=$!
  mesail_fylo_image_ready 2> "$work/second.err" &
  second=$!
  wait "$first"
  fylo_image_test_expect_status "the first of two gates succeeds" 0 "$?"
  wait "$second"
  fylo_image_test_expect_status "the second of two gates succeeds" 0 "$?"
  fylo_image_test_expect_status "only one of two gates attaches" 1 "$(cat "$work/first.err" "$work/second.err" | grep -c -F '=== fylo image: attaching')"
  fylo_image_test_expect_no_lock "the lock is released after two gates"
}

fylo_image_test_dead_atlas() {
  local work="$1" started finished status
  fylo_image_test_detach
  started="$(date +%s)"
  ( export MESAIL_FYLO_IMAGE_CHECK_SECONDS=1
    mesail_fylo_image_present() { sleep 5; }
    mesail_fylo_image_ready 2> "$work/dead.err" )
  status=$?
  finished="$(date +%s)"
  fylo_image_test_expect_status "a dead ATLAS fails" 1 "$status"
  fylo_image_test_expect_contains "a dead ATLAS is reported" "did not answer within 1 s" "$work/dead.err"
  fylo_image_test_expect_contains "a dead ATLAS names the fix" "is ATLAS mounted from CITADEL?" "$work/dead.err"
  fylo_image_test_expect_at_most "a dead ATLAS does not hold the run" 3 "$((finished - started))"
  fylo_image_test_expect_no_lock "the lock is released after a dead ATLAS"
}

fylo_image_test_missing_image_file() {
  local work="$1" status
  ( export MESAIL_FYLO_IMAGE="$work/no-such.sparsebundle"
    mesail_fylo_image_ready 2> "$work/missing.err" )
  status=$?
  fylo_image_test_expect_status "a missing image file fails" 1 "$status"
  fylo_image_test_expect_contains "a missing image file is named" "no-such.sparsebundle cannot be read" "$work/missing.err"
}

fylo_image_test_main() {
  local work
  work="$(mktemp -d)"
  export MESAIL_FYLO_IMAGE="$work/test.sparsebundle" MESAIL_FYLO_IMAGE_MOUNT="$work/mount" MESAIL_FYLO_IMAGE_LOCK="$work/locks/fylo-image.lock"
  fylo_image_test_make_image "$MESAIL_FYLO_IMAGE" || fylo_image_test_fail "setup" "hdiutil could not make $MESAIL_FYLO_IMAGE"
  fylo_image_test_attaches_when_missing "$work"
  fylo_image_test_keeps_a_mounted_image "$work"
  fylo_image_test_two_gates_at_once "$work"
  fylo_image_test_dead_atlas "$work"
  fylo_image_test_missing_image_file "$work"
  fylo_image_test_detach
  rm -rf "$work"
  echo "fylo-image: all checks passed"
}

fylo_image_test_main
