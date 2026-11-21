#!/bin/bash
set -u

utc_time() {
  local now="$EPOCHREALTIME" fraction
  fraction="${now#*.}"
  export TZ=UTC
  printf '%(%Y-%m-%dT%H:%M:%S)T.%sZ' "${now%.*}" "${fraction:0:3}"
}

subcommand() {
  if [[ "${1:-}" =~ ^[a-z][a-z-]{0,31}$ ]]; then echo "$1"; return; fi
  echo other
}

start_line() {
  printf '%s fylo-start pid=%s subcommand=%s\n' "$(utc_time)" "$$" "$(subcommand "${1:-}")"
}

stamp_lines() {
  local line
  while IFS= read -r line; do
    printf '%s %s\n' "$(utc_time)" "$line"
  done
}

main() {
  local log="/tmp/fylo-trace-$$.log" line
  line="$(start_line "${1:-}")"
  printf '%s\n' "$line" >> "$log"
  printf '%s\n' "$line" >> /tmp/fylo-trace-starts.log
  exec 2> >(stamp_lines >> "$log")
  exec /usr/local/lib/mesail/fylo.real "$@"
}

main "$@"
