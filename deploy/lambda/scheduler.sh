#!/bin/sh
set -eu

token_request() {
  printf '{"clientId":"mesail-service","clientSecret":"%s","domainId":"*"}' "$MESAIL_SCHEDULER_CLIENT_SECRET"
}

service_token() {
  token_request | curl --fail --silent --show-error --request POST --header 'Content-Type: application/json' \
    --data-binary @- "$MESAIL_SCHEDULER_TOKEN_URL" \
    | sed -n 's/.*"accessToken" *: *"\([^"]*\)".*/\1/p'
}

tick_seconds() {
  echo "${MESAIL_SCHEDULER_SECONDS:-60}"
}

repeat_seconds() {
  half=$(($(tick_seconds) / 2)) rest=$(($(tick_seconds) - 10))
  if [ "$rest" -gt "$half" ]; then echo "$rest"; else echo "$half"; fi
}

all_paths() {
  echo "$MESAIL_SCHEDULER_PATHS" | tr ',' ' '
}

now_seconds() {
  date -u +%s
}

clock() {
  date -u +%H:%M:%S
}

answer_summary() {
  head -c 200 "$1" 2> /dev/null | tr -d '\n'
}

call_route() {
  token="$1" path="$2" body="$3"
  code="$(curl --silent --output "$body" --write-out '%{http_code}' --max-time 120 --request POST \
    --header "Authorization: Bearer $token" "$MESAIL_SCHEDULER_BASE$path" || true)"
  echo "$(clock) POST $path -> ${code:-000} $(answer_summary "$body")"
  [ "${code:-000}" = 200 ] && grep -qs '"more" *: *true' "$body"
}

past_deadline() {
  [ "$(now_seconds)" -ge "$deadline" ]
}

call_round() {
  round="$1" remaining=""
  for path in $active; do
    if [ "$round" -gt 1 ] && past_deadline; then continue; fi
    echo "$path" >> "$work/calls"
    if call_route "$token" "$path" "$work/body"; then remaining="$remaining $path"; fi
  done
  active="$remaining"
}

report_calls() {
  counts=""
  for path in $(all_paths); do
    counts="$counts $path=$(grep -cxF -- "$path" "$work/calls" || true)"
  done
  echo "$(clock) calls this tick:$counts"
}

call_until_done() {
  active="$(all_paths)" round=1
  while [ -n "$active" ]; do
    call_round "$round"
    round=$((round + 1))
  done
}

retry_seconds() {
  if [ "$(tick_seconds)" -gt 60 ]; then echo 60; else tick_seconds; fi
}

run_tick() {
  token="$(service_token || true)"
  if [ -z "$token" ]; then echo "$(clock) no service token yet; trying again in $(retry_seconds) s"; return 1; fi
  deadline=$(($(now_seconds) + $(repeat_seconds)))
  work="$(mktemp -d)"
  : > "$work/calls"
  call_until_done
  report_calls
  rm -rf "$work"
}

wait_for_next_tick() {
  left=$(($1 + $2 - $(now_seconds)))
  if [ "$left" -gt 0 ]; then sleep "$left"; fi
}

main() {
  echo "scheduler: every $(tick_seconds) s, calls again while more: true until $(repeat_seconds) s: $MESAIL_SCHEDULER_PATHS"
  while true; do
    started="$(now_seconds)"
    if run_tick; then wait_for_next_tick "$started" "$(tick_seconds)"; else wait_for_next_tick "$started" "$(retry_seconds)"; fi
  done
}

main
