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

mesail_lambda_services() {
  mesail_stack '.services[] | select((.kind // "lambda") == "lambda") | .name'
}

mesail_is_container() {
  [ "$(mesail_service_field "$1" 'kind // "lambda"')" = container ]
}

mesail_service_field() {
  jq -r --arg name "$1" ".services[] | select(.name == \$name) | .$2" "$(mesail_stack_file)"
}

mesail_secret_value() {
  case "$1" in
    hex) openssl rand -hex 32 ;;
    random-32-base64) openssl rand -base64 32 ;;
    p256-pkcs8-base64) openssl genpkey -algorithm EC -pkeyopt ec_paramgen_curve:P-256 | openssl pkcs8 -topk8 -nocrypt -outform DER | base64 | tr -d '\n' ;;
    *) echo "unknown secret kind: $1" >&2; return 1 ;;
  esac
}

mesail_generate_secrets() {
  local file name kind
  file="$(mktemp "${TMPDIR:-/tmp}/mesail-secrets.XXXXXX")"
  chmod 600 "$file"
  while read -r name kind; do
    echo "$name=$(mesail_secret_value "$kind")" >> "$file"
  done < <(mesail_stack '.generatedSecrets | to_entries[] | "\(.key) \(.value)"')
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

mesail_app_settings() {
  jq -r --arg app "$1" '.appSettings[$app] // {} | to_entries[] | "\(.key)=\(.value)"' "$(mesail_stack_file)"
}

mesail_service_answers() {
  curl --fail --silent "$1/api/v1/$2/health" > /dev/null
}

mesail_post_json() {
  local url="$1" token="${2:-}"
  curl --fail-with-body --silent --show-error --request POST --header 'Content-Type: application/json' \
    ${token:+--header "Authorization: Bearer $token"} --data-binary @- "$url"
}

mesail_service_token() {
  local base="$1"
  jq -n '{clientId: "mesail-service", clientSecret: env.MESAIL_SERVICE_CLIENT_SECRET, domainId: "*"}' \
    | mesail_post_json "$base/api/v1/identity/service-token" | jq -r '.accessToken // empty'
}

mesail_sign_in() {
  local base="$1" login="$2" password="$3"
  jq -n --arg login "$login" --arg password "$password" '{login: $login, password: $password}' \
    | mesail_post_json "$base/api/v1/identity/session" | jq -r '.accessToken // empty'
}

mesail_person_field() {
  jq -r --arg name "$1" ".people[] | select(.username == \$name) | .$2" "$(mesail_stack_file)"
}

mesail_sign_in_person() {
  local base="$1" username="$2"
  mesail_sign_in "$base" "$username" "$(mesail_person_field "$username" password)"
}

mesail_operator() {
  mesail_stack '.people[] | select(.operator) | .username'
}

mesail_person_seed() {
  jq -c --arg name "$1" '.people[] | select(.username == $name)
    | {personId, username, displayName, memberships: [.memberships[] | {domainId, admin}]}' "$(mesail_stack_file)"
}

mesail_directory_call() {
  local base="$1" token="$2" method="$3" path="$4" body="${5:-}"
  curl --silent --output /dev/null --write-out '%{http_code}' --request "$method" \
    --header "Authorization: Bearer $token" --header 'Content-Type: application/json' \
    ${body:+--data "$body"} "$base/api/v1/directory$path"
}

mesail_add_domain() {
  local base="$1" token="$2" domain="$3"
  echo "add domain $domain: $(mesail_directory_call "$base" "$token" POST /domains "{\"name\":\"$domain\"}")"
}

mesail_seed_domain() {
  local base="$1" token="$2" domain="$3"
  mesail_add_domain "$base" "$token" "$domain"
  echo "verify domain $domain: $(mesail_directory_call "$base" "$token" POST "/domains/$domain/verify")"
}

mesail_seed_person() {
  local base="$1" token="$2" username="$3" code
  code="$(mesail_directory_call "$base" "$token" POST /people/seed "$(mesail_person_seed "$username")")"
  echo "seed person $username: $code"
  if [ "$code" != 200 ]; then echo "warning: the directory did not seed $username, so its tests fail"; fi
}

mesail_require_token() {
  if [ -n "$2" ]; then return 0; fi
  echo "no $1 from identity, so the directory is not seeded"
  return 1
}

mesail_seed_people() {
  local base="$1" token="$2" username
  for username in $(mesail_stack '.people[] | select(.operator | not) | .username'); do
    mesail_seed_person "$base" "$token" "$username"
  done
}

mesail_seed_domains() {
  local base="$1" token="$2" domain
  for domain in $(mesail_stack '.hostedDomains[]'); do
    mesail_seed_domain "$base" "$token" "$domain"
  done
}

mesail_seed_directory() {
  local base="$1" service operator
  service="$(mesail_service_token "$base" || true)"
  mesail_require_token "service token" "$service" || return 1
  mesail_seed_person "$base" "$service" "$(mesail_operator)"
  operator="$(mesail_sign_in_person "$base" "$(mesail_operator)" || true)"
  mesail_require_token "operator token" "$operator" || return 1
  mesail_seed_domains "$base" "$operator"
  mesail_seed_people "$base" "$service"
}
