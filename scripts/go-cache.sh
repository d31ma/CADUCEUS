#!/usr/bin/env bash
# shellcheck shell=bash

mesail_default_go_cache() {
  if [ "$(uname -s)" = Darwin ]; then echo "$HOME/Library/Caches/go-build-mesail"; fi
}

mesail_fallback_go_cache() {
  echo "${TMPDIR:-/tmp}/go-build-mesail-fallback"
}

mesail_go_cache_minimum_free_gib() {
  echo "${MESAIL_GOCACHE_MIN_FREE_GB:-35}"
}

mesail_go_cache_usable() {
  mkdir -p "$1" 2> /dev/null && [ -d "$1" ] && [ -w "$1" ]
}

mesail_choose_go_cache() {
  local folder="$1" fallback
  if mesail_go_cache_usable "$folder"; then echo "$folder"; return 0; fi
  fallback="$(mesail_fallback_go_cache)"
  echo "=== go cache: $folder is not usable (disk not mounted or link broken); using $fallback" >&2
  mkdir -p "$fallback"
  echo "$fallback"
}

mesail_free_gib() {
  df -Pk "$1/" | awk 'NR == 2 { print int($4 / 1048576) }'
}

mesail_go_cache_maximum_gib() {
  echo "${MESAIL_GOCACHE_MAX_GB:-10}"
}

mesail_go_cache_kib() {
  du -sk "$1/" | awk '{ print $1 }'
}

mesail_go_cache_over_maximum() {
  [ "$(mesail_go_cache_kib "$1")" -gt "$(($(mesail_go_cache_maximum_gib) * 1048576))" ]
}

mesail_remove_old_go_cache_entries() {
  find "$1/" -mindepth 2 -maxdepth 2 -type f -name '*-[ad]' -mmin "+${2:-60}" -print -delete | wc -l | tr -d ' '
}

mesail_trim_for_free_space() {
  local folder="$1" free minimum removed
  free="$(mesail_free_gib "$folder")"
  minimum="$(mesail_go_cache_minimum_free_gib)"
  if [ "$free" -ge "$minimum" ]; then return 0; fi
  removed="$(mesail_remove_old_go_cache_entries "$folder")"
  echo "=== go cache: $free GiB free (under $minimum GiB); removed $removed entries unused for over 1 hour from $folder; now $(mesail_free_gib "$folder") GiB free" >&2
}

mesail_trim_for_size() {
  local folder="$1" removed
  if ! mesail_go_cache_over_maximum "$folder"; then return 0; fi
  removed="$(mesail_remove_old_go_cache_entries "$folder")"
  echo "=== go cache: over $(mesail_go_cache_maximum_gib) GiB; removed $removed entries unused for over 1 hour; now $(($(mesail_go_cache_kib "$folder") / 1024)) MiB" >&2
  mesail_clean_when_still_over "$folder"
}

mesail_clean_when_still_over() {
  local folder="$1" removed
  if ! mesail_go_cache_over_maximum "$folder"; then return 0; fi
  removed="$(mesail_remove_old_go_cache_entries "$folder" 10)"
  echo "=== go cache: still over $(mesail_go_cache_maximum_gib) GiB; removed $removed entries unused for over 10 minutes; now $(($(mesail_go_cache_kib "$folder") / 1024)) MiB" >&2
}

mesail_trim_go_cache() {
  mesail_trim_for_size "$1"
  mesail_trim_for_free_space "$1"
}

mesail_add_trimpath() {
  case " ${GOFLAGS:-} " in
    *" -trimpath "*) return 0 ;;
  esac
  export GOFLAGS="${GOFLAGS:+$GOFLAGS }-trimpath"
}

mesail_use_local_go_cache() {
  local folder
  folder="${MESAIL_GOCACHE:-$(mesail_default_go_cache)}"
  if [ -z "$folder" ]; then return 0; fi
  folder="$(mesail_choose_go_cache "$folder")"
  mesail_trim_go_cache "$folder"
  mesail_add_trimpath
  export GOCACHE="$folder"
}
