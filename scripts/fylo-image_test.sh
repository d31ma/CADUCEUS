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

fylo_image_test_attach_read_only() {
  mkdir -p "$(mesail_fylo_image_mount_point)"
  hdiutil attach -quiet -readonly -nobrowse -noautoopen -mountpoint "$(mesail_fylo_image_mount_point)" "$(mesail_fylo_image_file)"
}

fylo_image_test_writable() {
  [ "$(mesail_fylo_image_write_problem)" = none ]
}

fylo_image_test_stops_when_read_only() {
  local work="$1" status
  fylo_image_test_detach
  fylo_image_test_attach_read_only || fylo_image_test_fail "read-only setup" "hdiutil could not attach the test image read-only"
  mesail_fylo_image_ready 2> "$work/read-only.err"
  status=$?
  fylo_image_test_expect_status "a read-only mount stops the gate" 1 "$status"
  fylo_image_test_expect_contains "a read-only mount is reported" "is mounted but cannot be written" "$work/read-only.err"
  fylo_image_test_expect_contains "a read-only mount is not re-attached" "does not re-attach a read-only image" "$work/read-only.err"
  fylo_image_test_expect_status "a read-only mount stays mounted" 0 "$(mesail_fylo_image_mounted; echo $?)"
  fylo_image_test_expect_no_lock "the lock is released after a read-only mount"
}

fylo_image_test_reattaches_after_io_error() {
  local work="$1" status
  fylo_image_test_detach
  mesail_fylo_image_ready 2> /dev/null || fylo_image_test_fail "io-error setup" "the test image could not be attached"
  ( fylo_image_test_real_detach() { hdiutil detach -quiet "$1"; }
    mesail_fylo_image_write_probe() {
      if [ -e "$work/detached" ]; then : > "$1/.probe" && rm -f "$1/.probe"; return; fi
      echo "touch: $1/.probe: Input/output error" > "$2"
      return 1
    }
    mesail_fylo_image_run_detach() { touch "$work/detached" && fylo_image_test_real_detach "$1"; }
    mesail_fylo_image_ready 2> "$work/io-error.err" )
  status=$?
  fylo_image_test_expect_status "an unused image with I/O errors is fixed" 0 "$status"
  fylo_image_test_expect_contains "the re-attach is reported" "gives I/O errors (old NFS handles) and nothing uses it" "$work/io-error.err"
  fylo_image_test_expect_contains "the image is attached again" "=== fylo image: attaching" "$work/io-error.err"
  fylo_image_test_expect_status "the image is writable after the re-attach" 0 "$(fylo_image_test_writable; echo $?)"
  fylo_image_test_expect_no_lock "the lock is released after the re-attach"
}

fylo_image_test_stops_after_io_error_when_busy() {
  local work="$1" holder status started finished mounted
  mesail_fylo_image_ready 2> /dev/null || fylo_image_test_fail "busy setup" "the test image could not be attached"
  (cd "$(mesail_fylo_image_mount_point)" && exec sleep 30) &
  holder=$!
  sleep 1
  started="$(date +%s)"
  ( mesail_fylo_image_write_probe() { echo "touch: $1/.probe: Input/output error" > "$2"; return 1; }
    mesail_fylo_image_ready 2> "$work/busy.err" )
  status=$?
  finished="$(date +%s)"
  mounted="$(mesail_fylo_image_mounted; echo $?)"
  kill "$holder" 2> /dev/null
  wait "$holder" 2> /dev/null
  fylo_image_test_expect_status "a busy image with I/O errors stops the gate" 1 "$status"
  fylo_image_test_expect_contains "a busy image with I/O errors is reported" "but is in use" "$work/busy.err"
  fylo_image_test_expect_contains "the process that uses it is listed" "sleep" "$work/busy.err"
  fylo_image_test_expect_status "a busy image is not detached" 0 "$mounted"
  fylo_image_test_expect_at_most "a busy image stops at once" 10 "$((finished - started))"
  fylo_image_test_expect_no_lock "the lock is released after a busy image"
}

fylo_image_test_stops_when_attached_read_only() {
  local work="$1" status
  fylo_image_test_detach
  ( mesail_fylo_image_run_attach() { mkdir -p "$1" && hdiutil attach -quiet -readonly -nobrowse -noautoopen -mountpoint "$1" "$2"; }
    mesail_fylo_image_ready 2> "$work/forced.err" )
  status=$?
  fylo_image_test_expect_status "an attach that stays read-only stops the gate" 1 "$status"
  fylo_image_test_expect_contains "an attach that stays read-only is reported" "is mounted but cannot be written" "$work/forced.err"
  fylo_image_test_expect_no_lock "the lock is released after a read-only attach"
}

fylo_image_test_missing_image_file() {
  local work="$1" status
  ( export MESAIL_FYLO_IMAGE="$work/no-such.sparsebundle"
    mesail_fylo_image_ready 2> "$work/missing.err" )
  status=$?
  fylo_image_test_expect_status "a missing image file fails" 1 "$status"
  fylo_image_test_expect_contains "a missing image file is named" "no-such.sparsebundle cannot be read" "$work/missing.err"
}

fylo_image_test_cleanup() {
  fylo_image_test_detach
  rm -rf "$1"
}

fylo_image_test_main() {
  local work
  work="$(mktemp -d)"
  # shellcheck disable=SC2064
  trap "fylo_image_test_cleanup '$work'" EXIT
  export MESAIL_FYLO_IMAGE="$work/test.sparsebundle" MESAIL_FYLO_IMAGE_MOUNT="$work/mount" MESAIL_FYLO_IMAGE_LOCK="$work/locks/fylo-image.lock"
  fylo_image_test_make_image "$MESAIL_FYLO_IMAGE" || fylo_image_test_fail "setup" "hdiutil could not make $MESAIL_FYLO_IMAGE"
  fylo_image_test_attaches_when_missing "$work"
  fylo_image_test_keeps_a_mounted_image "$work"
  fylo_image_test_two_gates_at_once "$work"
  fylo_image_test_stops_when_read_only "$work"
  fylo_image_test_reattaches_after_io_error "$work"
  fylo_image_test_stops_after_io_error_when_busy "$work"
  fylo_image_test_stops_when_attached_read_only "$work"
  fylo_image_test_dead_atlas "$work"
  fylo_image_test_missing_image_file "$work"
  echo "fylo-image: all checks passed"
}

fylo_image_test_main
