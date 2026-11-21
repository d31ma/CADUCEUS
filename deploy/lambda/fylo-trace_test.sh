#!/bin/bash
set -uo pipefail

fail() {
  echo "FAIL $1: $2" >&2
  exit 1
}

expect_line() {
  local name="$1" pattern="$2" file="$3"
  if ! grep -q -E -- "$pattern" "$file"; then fail "$name" "no line matching '$pattern' in $(cat "$file" 2> /dev/null)"; fi
  echo "ok   $name"
}

check_installed() {
  if [ ! -x /usr/local/lib/mesail/fylo.real ]; then fail "wrapper installed" "no fylo.real (image built without MESAIL_FYLO_TRACE=1)"; fi
  echo "ok   wrapper installed"
}

check_same_pid() {
  local pid
  fylo version > /dev/null 2>&1 &
  pid=$!
  wait "$pid"
  expect_line "start line with the same pid" "^[0-9T:.-]+Z fylo-start pid=$pid subcommand=version$" "/tmp/fylo-trace-$pid.log"
  expect_line "start line in the shared list" "pid=$pid subcommand=version" /tmp/fylo-trace-starts.log
}

check_arguments_hidden() {
  local pid
  fylo --root=/tmp/secret-value-x status > /dev/null 2>&1 &
  pid=$!
  wait "$pid"
  expect_line "first argument not a word is hidden" "pid=$pid subcommand=other$" "/tmp/fylo-trace-$pid.log"
  if grep -q secret-value-x "/tmp/fylo-trace-$pid.log"; then fail "arguments hidden" "an argument reached the log"; fi
  echo "ok   arguments hidden"
}

check_stderr_logged() {
  local pid
  fylo no-such-command > /dev/null 2> /tmp/visible-stderr &
  pid=$!
  wait "$pid"
  if [ -s /tmp/visible-stderr ]; then fail "stderr only to the log" "stderr also reached the caller"; fi
  expect_line "stderr lines have a time" "^[0-9T:.-]+Z [^f]" "/tmp/fylo-trace-$pid.log"
}

real_fylo_running() {
  local link
  for link in /proc/[0-9]*/exe; do
    if [ "$(readlink "$link" 2> /dev/null)" = /usr/local/lib/mesail/fylo.real ]; then return 0; fi
  done
  return 1
}

check_kill_leaves_nothing() {
  local pid
  mkdir -p /tmp/trace-test-root
  sleep 30 | fylo exec --loop --root /tmp/trace-test-root > /dev/null &
  pid=$!
  sleep 1
  kill -KILL "$pid"
  wait "$pid" 2> /dev/null
  sleep 1
  if kill -0 "$pid" 2> /dev/null; then fail "SIGKILL stops fylo" "pid $pid still runs"; fi
  if real_fylo_running; then fail "no orphan fylo" "a fylo.real process still runs"; fi
  echo "ok   SIGKILL leaves no fylo running"
}

main() {
  check_installed
  check_same_pid
  check_arguments_hidden
  check_stderr_logged
  check_kill_leaves_nothing
  echo "fylo-trace_test: all passed"
}

main "$@"
