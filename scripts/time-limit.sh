#!/usr/bin/env bash
# shellcheck shell=bash

mesail_probe_finished_in_time() {
  local pid="$1" limit="$2" tenths=0
  while kill -0 "$pid" 2> /dev/null; do
    if [ "$tenths" -ge "$((limit * 10))" ]; then return 1; fi
    sleep 0.1
    tenths=$((tenths + 1))
  done
}

mesail_report_stuck_probe() {
  local what="$1" limit="$2" pid="$3"
  kill "$pid" 2> /dev/null
  echo "=== $what did not answer within $limit s (is its disk mounted?); probe pid $pid left behind" >&2
  return 2
}

mesail_finishes_within() {
  local limit="$1" what="$2" pid
  shift 2
  "$@" > /dev/null 2>&1 < /dev/null &
  pid=$!
  if ! mesail_probe_finished_in_time "$pid" "$limit"; then
    mesail_report_stuck_probe "$what" "$limit" "$pid"
    return 2
  fi
  wait "$pid"
}
