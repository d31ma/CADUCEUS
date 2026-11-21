#!/usr/bin/env bash
# shellcheck shell=bash
# shellcheck source-path=SCRIPTDIR
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/time-limit.sh"

mesail_fylo_image_mount_point() {
  echo "${MESAIL_FYLO_IMAGE_MOUNT:-$HOME/Developer/AtlasFylo/mesail}"
}

mesail_fylo_image_file() {
  echo "${MESAIL_FYLO_IMAGE:-/Volumes/ATLAS/Mesail/mesail-fylo-test.sparsebundle}"
}

mesail_fylo_image_lock() {
  echo "${MESAIL_FYLO_IMAGE_LOCK:-$HOME/Library/Caches/mac-locks/mesail-fylo-image.lock}"
}

mesail_fylo_image_check_seconds() {
  echo "${MESAIL_FYLO_IMAGE_CHECK_SECONDS:-10}"
}

mesail_fylo_image_attach_seconds() {
  echo "${MESAIL_FYLO_IMAGE_ATTACH_SECONDS:-120}"
}

mesail_fylo_image_lock_wait_seconds() {
  echo "${MESAIL_FYLO_IMAGE_LOCK_WAIT_SECONDS:-180}"
}

mesail_fylo_image_busy_seconds() {
  echo "${MESAIL_FYLO_IMAGE_BUSY_SECONDS:-30}"
}

mesail_fylo_image_real_mount_point() {
  local mount_point parent
  mount_point="$(mesail_fylo_image_mount_point)"
  mkdir -p "$(dirname "$mount_point")"
  parent="$(cd "$(dirname "$mount_point")" && pwd -P)"
  echo "$parent/$(basename "$mount_point")"
}

mesail_fylo_image_mount_line() {
  mount | grep -F " on $(mesail_fylo_image_real_mount_point) ("
}

mesail_fylo_image_mounted() {
  mesail_fylo_image_mount_line > /dev/null
}


mesail_fylo_image_present() {
  [ -d "$1" ]
}

mesail_fylo_image_reachable() {
  local image
  image="$(mesail_fylo_image_file)"
  if mesail_finishes_within "$(mesail_fylo_image_check_seconds)" "the FYLO test image $image" mesail_fylo_image_present "$image"; then return 0; fi
  echo "=== fylo image: $image cannot be read (is ATLAS mounted from CITADEL?)" >&2
  return 1
}

mesail_fylo_image_run_attach() {
  mkdir -p "$1" && diskutil image attach --mountOptions nobrowse --mountPoint "$1" "$2"
}

mesail_fylo_image_attach() {
  local mount_point image
  mount_point="$(mesail_fylo_image_mount_point)"
  image="$(mesail_fylo_image_file)"
  echo "=== fylo image: attaching $image at $mount_point" >&2
  mesail_finishes_within "$(mesail_fylo_image_attach_seconds)" "diskutil image attach of $image" mesail_fylo_image_run_attach "$mount_point" "$image" || return 1
  mesail_fylo_image_mounted
}

mesail_fylo_image_write_probe() {
  local probe="$1/.mesail-write-probe-$$"
  : > "$probe" 2> "$2" && rm -f "$probe" 2>> "$2"
}

mesail_fylo_image_write_problem() {
  local mount_point errors status
  mount_point="$(mesail_fylo_image_mount_point)"
  errors="$(mktemp)"
  mesail_finishes_within "$(mesail_fylo_image_check_seconds)" "a write probe in $mount_point" mesail_fylo_image_write_probe "$mount_point" "$errors"
  status=$?
  mesail_fylo_image_name_problem "$status" "$errors"
  rm -f "$errors"
}

mesail_fylo_image_name_problem() {
  local status="$1" errors="$2"
  case "$status:$(cat "$errors")" in
    0:*) echo none ;;
    2:*) echo stuck ;;
    *"Input/output error"*) echo io-error ;;
    *) echo "not-writable: $(tr '\n' ' ' < "$errors")" ;;
  esac
}

mesail_fylo_image_stop_unwritable() {
  echo "=== fylo image: $(mesail_fylo_image_mount_point) is mounted but cannot be written ($1), so the gate stops; gates.sh does not re-attach a read-only image (ask Platform; macOS logs why under 'reOpenIfWritable')" >&2
  return 1
}

mesail_fylo_image_require_writable() {
  local problem
  problem="$(mesail_fylo_image_write_problem)"
  if [ "$problem" = none ]; then return 0; fi
  mesail_fylo_image_stop_unwritable "$problem"
}

mesail_fylo_image_list_users() {
  lsof -w +D "$1" > "$2" 2>&1
}

mesail_fylo_image_explain_users() {
  local status="$1" users="$2" mount_point
  if [ "$status" -eq 1 ]; then return 0; fi
  mount_point="$(mesail_fylo_image_mount_point)"
  echo "=== fylo image: $mount_point gives I/O errors (old NFS handles) but is in use (or lsof did not answer), so it is not re-attached; stop these processes, run: hdiutil detach $mount_point, then rerun the gate" >&2
  sed 's/^/    /' "$users" >&2
  return 1
}

mesail_fylo_image_unused() {
  local mount_point users status
  mount_point="$(mesail_fylo_image_mount_point)"
  users="$(mktemp)"
  mesail_finishes_within "$(mesail_fylo_image_busy_seconds)" "lsof of $mount_point" mesail_fylo_image_list_users "$mount_point" "$users"
  status=$?
  mesail_fylo_image_explain_users "$status" "$users"
  status=$?
  rm -f "$users"
  return "$status"
}

mesail_fylo_image_run_detach() {
  hdiutil detach -quiet "$1"
}

mesail_fylo_image_detach_broken() {
  local mount_point
  mount_point="$(mesail_fylo_image_mount_point)"
  echo "=== fylo image: $mount_point gives I/O errors (old NFS handles) and nothing uses it, so it is detached and attached again" >&2
  mesail_finishes_within "$(mesail_fylo_image_attach_seconds)" "hdiutil detach of $mount_point" mesail_fylo_image_run_detach "$mount_point"
}

mesail_fylo_image_reattach_broken() {
  local problem
  problem="$(mesail_fylo_image_write_problem)"
  if [ "$problem" = none ]; then return 0; fi
  if [ "$problem" != io-error ]; then mesail_fylo_image_stop_unwritable "$problem"; return 1; fi
  mesail_fylo_image_unused || return 1
  mesail_fylo_image_detach_broken || return 1
  mesail_fylo_image_attach_under_lock
}

mesail_fylo_image_owner_alive() {
  local owner
  owner="$(cat "$1/pid" 2> /dev/null)" || return 1
  kill -0 "$owner" 2> /dev/null
}

mesail_fylo_image_remove_dead_lock() {
  local lock="$1"
  if mesail_fylo_image_owner_alive "$lock"; then return 0; fi
  rm -rf "$lock"
}

mesail_fylo_image_take_lock() {
  local lock="$1" limit="$2" waited=0
  mkdir -p "$(dirname "$lock")"
  until mkdir "$lock" 2> /dev/null; do
    if [ "$waited" -ge "$limit" ]; then return 1; fi
    mesail_fylo_image_remove_dead_lock "$lock"
    sleep 1
    waited=$((waited + 1))
  done
  echo "$$" > "$lock/pid"
}

mesail_fylo_image_attach_under_lock() {
  if mesail_fylo_image_mounted; then mesail_fylo_image_require_writable; return; fi
  mesail_fylo_image_reachable || return 1
  mesail_fylo_image_attach || return 1
  mesail_fylo_image_require_writable
}

mesail_fylo_image_under_lock() {
  local lock status
  lock="$(mesail_fylo_image_lock)"
  if ! mesail_fylo_image_take_lock "$lock" "$(mesail_fylo_image_lock_wait_seconds)"; then
    echo "=== fylo image: another gate has held $lock for too long" >&2
    return 1
  fi
  "$@"
  status=$?
  rm -rf "$lock"
  return "$status"
}

mesail_fylo_image_check_mounted() {
  local problem
  problem="$(mesail_fylo_image_write_problem)"
  if [ "$problem" = none ]; then return 0; fi
  if [ "$problem" = io-error ]; then mesail_fylo_image_under_lock mesail_fylo_image_reattach_broken; return; fi
  mesail_fylo_image_stop_unwritable "$problem"
}

mesail_fylo_image_ready() {
  if mesail_fylo_image_mounted; then mesail_fylo_image_check_mounted; return; fi
  mesail_fylo_image_under_lock mesail_fylo_image_attach_under_lock
}
