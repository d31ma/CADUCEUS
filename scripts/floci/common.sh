#!/usr/bin/env bash
# shellcheck shell=bash

mesail_root() {
  cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd
}

mesail_stack_file() {
  echo "$(mesail_root)/deploy/floci/stack.json"
}

mesail_stack() {
  jq -r "$1" "$(mesail_stack_file)"
}

mesail_load_settings() {
  local file
  file="$(mesail_root)/.env"
  # shellcheck disable=SC1090
  if [ -f "$file" ]; then set -a; . "$file"; set +a; fi
  export MESAIL_PUBLIC_URL="${MESAIL_PUBLIC_URL:-http://127.0.0.1:18080}"
  export MESAIL_ROUTER_PORT="${MESAIL_ROUTER_PORT:-18080}"
  export MESAIL_KEYCLOAK_PORT="${MESAIL_KEYCLOAK_PORT:-18180}"
  export MESAIL_ISSUER_ORIGIN="${MESAIL_ISSUER_ORIGIN:-https://mesail.localhost}"
  export MESAIL_FLOCI_PORT="${MESAIL_FLOCI_PORT:-4566}"
  export MESAIL_FLOCI_URL="${MESAIL_FLOCI_URL:-http://127.0.0.1:$MESAIL_FLOCI_PORT}"
  export MESAIL_KEYCLOAK_URL="${MESAIL_KEYCLOAK_URL:-http://127.0.0.1:$MESAIL_KEYCLOAK_PORT/idp}"
  export MESAIL_ROUTER_URL="${MESAIL_ROUTER_URL:-http://127.0.0.1:$MESAIL_ROUTER_PORT}"
  export MESAIL_IMAGE_TAG="${MESAIL_IMAGE_TAG:-dev}"
  export AWS_ACCESS_KEY_ID=test AWS_SECRET_ACCESS_KEY=test AWS_REGION=us-east-1 AWS_PAGER=""
  unset AWS_PROFILE AWS_SESSION_TOKEN
  mesail_log_dir > /dev/null
}

mesail_log_dir() {
  if [ -n "${MESAIL_LOG_DIR:-}" ]; then echo "$MESAIL_LOG_DIR"; return; fi
  MESAIL_LOG_DIR="$HOME/Library/Logs/local/CADUCEUS/${MESAIL_OWNER:-floci}/floci-$(date +%Y%m%d-%H%M%S)"
  mkdir -p "$MESAIL_LOG_DIR"
  export MESAIL_LOG_DIR
  echo "$MESAIL_LOG_DIR"
}

mesail_step() {
  local name="$1" code
  shift
  echo "=== $name $(date +%H:%M:%S)"
  "$@" 2>&1 | tee -a "$(mesail_log_dir)/$name.log"
  code="${PIPESTATUS[0]}"
  echo "=== $name rc=$code"
  return "$code"
}

mesail_aws() {
  aws --endpoint-url "$MESAIL_FLOCI_URL" --region "$AWS_REGION" --no-cli-pager --output json "$@"
}

mesail_wait_for() {
  local what="$1" seconds="$2" waited=0
  shift 2
  until "$@" > /dev/null 2>&1; do
    if [ "$waited" -ge "$seconds" ]; then echo "timed out after ${seconds} s waiting for $what"; return 1; fi
    sleep 2
    waited=$((waited + 2))
  done
  echo "$what is ready after ${waited} s"
}

mesail_services() {
  mesail_stack '.services[].name'
}

mesail_service_field() {
  jq -r --arg name "$1" ".services[] | select(.name == \$name) | .$2" "$(mesail_stack_file)"
}

mesail_generate_secrets() {
  local file name
  file="$(mktemp "${TMPDIR:-/tmp}/mesail-secrets.XXXXXX")"
  chmod 600 "$file"
  for name in $(mesail_stack '.generatedSecrets[]'); do
    echo "$name=$(openssl rand -hex 32)" >> "$file"
  done
  echo "$file"
}

mesail_load_secrets() {
  if [ -z "${MESAIL_SECRETS_FILE:-}" ]; then echo "MESAIL_SECRETS_FILE is not set" >&2; return 1; fi
  set -a
  # shellcheck disable=SC1090
  . "$MESAIL_SECRETS_FILE"
  set +a
}

mesail_dotted() {
  echo "${1#YON_}" | tr 'A-Z_' 'a-z.'
}

mesail_app_uses() {
  local folder="$1" name="$2"
  grep -rqsE --exclude-dir=.tachyon --exclude-dir=dist -e "$name" -e "$(mesail_dotted "$name")" "$folder"
}

mesail_app_secrets() {
  local folder="$1" name source
  while read -r name source; do
    if mesail_app_uses "$folder" "$name"; then echo "$name=${!source}"; fi
  done < <(mesail_stack '.secrets | to_entries[] | "\(.key) \(.value)"')
}
