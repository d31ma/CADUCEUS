#!/usr/bin/env bash
# shellcheck shell=bash
# shellcheck source-path=SCRIPTDIR
set -uo pipefail

capture_test_folder() {
  cd "$(dirname "$0")" && pwd
}

capture_test_fail() {
  echo "FAIL $1: $2" >&2
  exit 1
}

capture_test_expect_contains() {
  local name="$1" text="$2" file="$3"
  if ! grep -q -F -- "$text" "$file"; then capture_test_fail "$name" "'$text' not in $file"; fi
  echo "ok   $name"
}

capture_test_expect_file() {
  local name="$1" file="$2"
  if [ ! -s "$file" ]; then capture_test_fail "$name" "missing or empty: $file"; fi
  echo "ok   $name"
}

capture_test_line() {
  grep -n -F -- "$1" "$2" | head -1 | cut -d: -f1
}

capture_test_expect_before() {
  local name="$1" first="$2" second="$3" file="$4" one two
  one="$(capture_test_line "$first" "$file")"
  two="$(capture_test_line "$second" "$file")"
  if [ -z "$one" ] || [ -z "$two" ] || [ "$one" -ge "$two" ]; then
    capture_test_fail "$name" "'$first' (line ${one:-none}) must come before '$second' (line ${two:-none})"
  fi
  echo "ok   $name"
}

capture_test_write_curl() {
  cat > "$1/curl" << 'FAKE'
#!/usr/bin/env bash
output=""
previous=""
for argument in "$@"; do
  if [ "$previous" = --output ]; then output="$argument"; fi
  previous="$argument"
done
case "$*" in
  *service-token*) echo "curl token" >> "$FAKE_CALLS"; echo '{"accessToken":"fake-token"}' ;;
  *people/seed*)
    count=$(($(cat "$FAKE_STATE/seeds" 2> /dev/null || echo 0) + 1))
    echo "$count" > "$FAKE_STATE/seeds"
    echo "curl seed $count" >> "$FAKE_CALLS"
    if [ "$count" -le "$FAKE_FAILURES" ]; then
      echo '{"code":"unavailable","message":"[MS1048]: the directory store is not available"}' > "$output"
      printf '503 time=5.100000s'
    else
      echo '{}' > "$output"
      printf '200 time=0.100000s'
    fi ;;
esac
FAKE
}

capture_test_write_docker() {
  cat > "$1/docker" << 'FAKE'
#!/usr/bin/env bash
echo "docker $*" >> "$FAKE_CALLS"
container=floci-aws-mesail-directory-1a2b3c4d
fake_root() {
  mkdir -p "$FAKE_STATE/root/directory"
  echo data > "$FAKE_STATE/root/directory/file"
  echo '{"owner":"fake"}' > "$FAKE_STATE/root/.directory.fylo-root-owner.lock.json"
  tar -C "$FAKE_STATE/root" -cf - directory .directory.fylo-root-owner.lock.json
}
fake_copy() {
  case "$1" in
    *fylo-trace-starts.log) echo "2026-11-21T10:00:00.000Z fylo-start pid=42 subcommand=exec" > "$2" ;;
    *fylo-trace-42.log) echo "2026-11-21T10:00:00.000Z fylo-start pid=42 subcommand=exec" > "$2" ;;
    *fylo-trace-start.log) echo "2026-11-21T09:59:59.000Z snapshot root=/mnt/data/directory bytes=10240 seconds=0.010" > "$2" ;;
    *fylo-root-at-start.tar) fake_root > "$2" ;;
    *) exit 1 ;;
  esac
}
case "$1" in
  ps) echo "2026-11-21 10:00:00 +0000 UTC|$container"; echo "2026-11-21 10:00:00 +0000 UTC|floci-aws-mesail-mail-in-99999999" ;;
  inspect) if [[ "$*" == *Config.Env* ]]; then printf 'PATH=/usr/bin\nFYLO_EXAMPLE=hidden\n'; else echo "/$container created=x state=exited"; fi ;;
  cp) fake_copy "$2" "$3" ;;
  volume) echo floci-aws-efs-fsap-1-abc ;;
  run) if [[ "$*" == *capture-timing* ]]; then cat > /dev/null; echo "step start+handshake ms=12 ok=true code=none"; else fake_root; fi ;;
  logs) echo "container log line" ;;
esac
FAKE
}

capture_test_write_logger() {
  local file="$1" name="$2" answer="$3"
  cat > "$file" << FAKE
#!/usr/bin/env bash
echo "$name \$*" >> "\$FAKE_CALLS"
$answer
FAKE
}

capture_test_write_fakes() {
  local bin="$1"
  mkdir -p "$bin"
  capture_test_write_curl "$bin"
  capture_test_write_docker "$bin"
  capture_test_write_logger "$bin/sleep" sleep ""
  capture_test_write_logger "$bin/aws" aws "echo fsap-1"
  chmod 755 "$bin"/*
}

capture_test_run_seed() {
  local work="$1" failures="$2"
  ( export PATH="$work/bin:$PATH" FAKE_CALLS="$work/calls.log" FAKE_STATE="$work/state" FAKE_FAILURES="$failures"
    export MESAIL_LOG_DIR="$work/logs" MESAIL_SECRETS_FILE="$work/secrets" MESAIL_GOCACHE="$work/gocache"
    export MESAIL_FLOCI_URL=http://127.0.0.1:1 MESAIL_IMAGE_TAG=dev AWS_REGION=us-east-1
    . "$(capture_test_folder)/common.sh"
    mesail_seed_person http://127.0.0.1:1 fake-token operator ) > "$work/seed.log" 2>&1
}

capture_test_setup() {
  local work="$1"
  mkdir -p "$work/state" "$work/logs"
  : > "$work/secrets"
  echo "--file docker-compose.yml" > "$work/logs/compose-profile"
  capture_test_write_fakes "$work/bin"
}

capture_test_capture_before_retry() {
  local work="$1/before" folder
  capture_test_setup "$work"
  capture_test_run_seed "$work" 1
  folder="$(find "$work/logs" -maxdepth 1 -type d -name 'capture-*' | head -1)"
  capture_test_expect_contains "first try logged with its number" "seed person operator: 503 time=5.100000s code=unavailable number=MS1048" "$work/seed.log"
  capture_test_expect_contains "retry counted" "retry 1 of 3 in 5 s" "$work/seed.log"
  capture_test_expect_contains "passes after the retry" "(after retry 1 of 3)" "$work/seed.log"
  capture_test_expect_before "scheduler stopped before the retry" "docker compose" "curl seed 2" "$work/calls.log"
  capture_test_expect_before "app stopped before the retry" "docker stop --time 10 floci-aws-mesail-directory-1a2b3c4d" "curl seed 2" "$work/calls.log"
  capture_test_expect_before "fylo timed before the retry" "capture-timing" "curl seed 2" "$work/calls.log"
  capture_test_expect_file "containers.txt" "$folder/containers.txt"
  capture_test_expect_file "root-after.tar" "$folder/root-after.tar"
  capture_test_expect_file "owner file" "$folder/owner-files/.directory.fylo-root-owner.lock.json"
  capture_test_expect_file "start snapshot copied" "$folder/fylo-trace/floci-aws-mesail-directory-1a2b3c4d/fylo-root-at-start.tar"
  capture_test_expect_file "trace log copied" "$folder/fylo-trace/floci-aws-mesail-directory-1a2b3c4d/fylo-trace-42.log"
  capture_test_expect_contains "timing on the start snapshot" "start snapshot floci-aws-mesail-directory-1a2b3c4d/fylo-root-at-start.tar" "$folder/timing.txt"
  capture_test_expect_contains "volume mounted read only" "floci-aws-efs-fsap-1-abc:/data:ro" "$work/calls.log"
  capture_test_expect_contains "scratch root in its own volume" "mesail-op8-capture:/mnt/data --entrypoint /bin/bash" "$work/calls.log"
  if grep -q 'floci-aws-efs-fsap-1-abc:/mnt/data' "$work/calls.log"; then capture_test_fail "app volume never written" "mounted read-write"; fi
  capture_test_expect_contains "scheduler started again" "start scheduler scheduler-daily" "$work/calls.log"
  if grep -q 'mail-in' "$work/calls.log"; then capture_test_fail "only the app's containers" "a mail-in container was touched"; fi
  if grep -q 'hidden' "$work/seed.log" "$folder/timing.txt"; then capture_test_fail "no FYLO values in logs" "a FYLO_ value was written"; fi
  if grep -q -E '^docker (rm|volume rm|compose .* (down|rm))' "$work/calls.log"; then capture_test_fail "never removes" "a remove command ran"; fi
  echo "ok   never removes or prints FYLO values"
}

capture_test_all_fail() {
  local work="$1/fail"
  capture_test_setup "$work"
  capture_test_run_seed "$work" 9
  capture_test_expect_contains "third retry logged" "retry 3 of 3 in 5 s" "$work/seed.log"
  capture_test_expect_contains "warning stays" "warning: the directory did not seed operator" "$work/seed.log"
  if [ "$(grep -c 'docker stop' "$work/calls.log")" != 1 ]; then capture_test_fail "one capture per run" "$(grep -c 'docker stop' "$work/calls.log") stops"; fi
  echo "ok   one capture per run"
}

capture_test_other_error() {
  local work="$1/other"
  capture_test_setup "$work"
  sed -i.bak 's/MS1048/MS1001/' "$work/bin/curl"
  capture_test_run_seed "$work" 1
  if grep -q 'docker' "$work/calls.log" 2> /dev/null; then capture_test_fail "no capture for other numbers" "docker was called"; fi
  capture_test_expect_contains "other numbers still retry" "retry 1 of 3" "$work/seed.log"
}

capture_test_native_run() {
  local work="$1/native"
  capture_test_setup "$work"
  rm -f "$work/logs/compose-profile"
  capture_test_run_seed "$work" 1
  if grep -q 'docker' "$work/calls.log" 2> /dev/null; then capture_test_fail "no capture in the native run" "docker was called"; fi
  capture_test_expect_contains "native run says why" "capture: only in the Floci stack" "$work/seed.log"
}

capture_test_native_limit() {
  local limit
  # shellcheck disable=SC2016
  limit="$(env -u MESAIL_ROUTER_RATE_LIMIT MESAIL_GOCACHE="$1/gocache" MESAIL_LOG_DIR="$1/limit" bash -c \
    '. "$1/common.sh"; mesail_load_settings > /dev/null; echo "${MESAIL_ROUTER_RATE_LIMIT:-unset}"' _ "$(capture_test_folder)")"
  if [ "$limit" != unset ]; then capture_test_fail "native limit unchanged" "mesail_load_settings set $limit"; fi
  echo "ok   the 3000 sign-in limit is for the Floci stack only"
}

main() {
  local work
  work="$(mktemp -d "${TMPDIR:-/tmp}/mesail-capture-test.XXXXXX")"
  capture_test_capture_before_retry "$work"
  capture_test_all_fail "$work"
  capture_test_other_error "$work"
  capture_test_native_run "$work"
  capture_test_native_limit "$work"
  rm -rf "$work"
  echo "capture_test: all passed"
}

main "$@"
