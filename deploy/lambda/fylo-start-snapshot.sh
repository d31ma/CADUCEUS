#!/bin/bash
set -eu

limit_bytes() {
  echo 209715200
}

utc_time() {
  date -u +%Y-%m-%dT%H:%M:%S.%3NZ
}

root_bytes() {
  du -sb "$1" | cut -f1
}

seconds_since() {
  awk -v started="$1" -v now="$(date +%s.%N)" 'BEGIN { printf "%.3f", now - started }'
}

owner_files() {
  (cd "$1" && find . -maxdepth 1 -name ".$2.fylo-root-owner.lock*" | sed 's|^\./||')
}

write_snapshot() {
  local root="$1" parent name started
  parent="$(dirname "$root")"
  name="$(basename "$root")"
  started="$(date +%s.%N)"
  # shellcheck disable=SC2046
  tar -C "$parent" -cf /tmp/fylo-root-at-start.tar "$name" $(owner_files "$parent" "$name")
  echo "$(utc_time) snapshot root=$root bytes=$(stat -c %s /tmp/fylo-root-at-start.tar) seconds=$(seconds_since "$started")" >> /tmp/fylo-trace-start.log
}

skip_line() {
  echo "$(utc_time) snapshot skipped root=$1: $2" >> /tmp/fylo-trace-start.log
}

main() {
  local root="${1:-}" size
  if [ "${MESAIL_FYLO_TRACE:-0}" != 1 ]; then return 0; fi
  if [ -z "$root" ] || [ ! -d "$root" ]; then skip_line "${root:-none}" "no root folder"; return 0; fi
  size="$(root_bytes "$root")"
  if [ "$size" -gt "$(limit_bytes)" ]; then skip_line "$root" "root is $size bytes, over $(limit_bytes)"; return 0; fi
  write_snapshot "$root" || skip_line "$root" "tar failed"
}

main "$@"
