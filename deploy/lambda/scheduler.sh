#!/bin/sh
set -eu

service_token() {
  curl --fail --silent --show-error "$MESAIL_SCHEDULER_TOKEN_URL" \
    --data-urlencode grant_type=client_credentials --data-urlencode client_id=mesail-service \
    --data-urlencode "client_secret=$MESAIL_SCHEDULER_CLIENT_SECRET" \
    | sed -n 's/.*"access_token":"\([^"]*\)".*/\1/p'
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
