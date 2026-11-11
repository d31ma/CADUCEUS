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
  export MESAIL_KEYCLOAK_ADMIN="${MESAIL_KEYCLOAK_ADMIN:-admin}"
  export MESAIL_KEYCLOAK_ADMIN_PASSWORD="${MESAIL_KEYCLOAK_ADMIN_PASSWORD:-mesail-dev-admin}"
  export MESAIL_SEED_PASSWORD="${MESAIL_SEED_PASSWORD:-mesail-dev-password}"
  export MESAIL_SERVICE_SECRET="${MESAIL_SERVICE_SECRET:-mesail-dev-service-secret}"
  export MESAIL_FLOCI_PORT="${MESAIL_FLOCI_PORT:-4566}"
  export MESAIL_FLOCI_URL="${MESAIL_FLOCI_URL:-http://127.0.0.1:$MESAIL_FLOCI_PORT}"
  export MESAIL_KEYCLOAK_URL="${MESAIL_KEYCLOAK_URL:-http://127.0.0.1:$MESAIL_KEYCLOAK_PORT}"
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
