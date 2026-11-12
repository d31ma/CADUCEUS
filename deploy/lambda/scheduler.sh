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

call_route() {
  token="$1" path="$2"
  code="$(curl --silent --output /dev/null --write-out '%{http_code}' --max-time 120 --request POST \
    --header "Authorization: Bearer $token" "$MESAIL_SCHEDULER_BASE$path" || echo 000)"
  echo "$(date -u +%H:%M:%S) POST $path -> $code"
}

run_once() {
  token="$(service_token || true)"
  if [ -z "$token" ]; then echo "$(date -u +%H:%M:%S) no service token yet"; return 0; fi
  for path in $(echo "$MESAIL_SCHEDULER_PATHS" | tr ',' ' '); do
    call_route "$token" "$path"
  done
}

main() {
  echo "scheduler: every ${MESAIL_SCHEDULER_SECONDS:-60} s: $MESAIL_SCHEDULER_PATHS"
  while true; do
    run_once
    sleep "${MESAIL_SCHEDULER_SECONDS:-60}"
  done
}

main
