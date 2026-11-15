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

mesail_fylo_image_real_mount_point() {
  local mount_point parent
  mount_point="$(mesail_fylo_image_mount_point)"
  mkdir -p "$(dirname "$mount_point")"
  parent="$(cd "$(dirname "$mount_point")" && pwd -P)"
  echo "$parent/$(basename "$mount_point")"
}

mesail_fylo_image_mounted() {
  mount | grep -q -F " on $(mesail_fylo_image_real_mount_point) ("
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
  if mesail_fylo_image_mounted; then return 0; fi
  mesail_fylo_image_reachable || return 1
  mesail_fylo_image_attach
}

mesail_fylo_image_attach_once() {
  local lock status
  lock="$(mesail_fylo_image_lock)"
  if ! mesail_fylo_image_take_lock "$lock" "$(mesail_fylo_image_lock_wait_seconds)"; then
    echo "=== fylo image: another gate has held $lock for too long" >&2
    return 1
  fi
  mesail_fylo_image_attach_under_lock
  status=$?
  rm -rf "$lock"
  return "$status"
}

mesail_fylo_image_ready() {
  if mesail_fylo_image_mounted; then return 0; fi
  mesail_fylo_image_attach_once
}
