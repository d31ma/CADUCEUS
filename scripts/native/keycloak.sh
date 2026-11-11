#!/usr/bin/env bash
# shellcheck shell=bash

keycloak_tools() {
  echo "${MESAIL_KEYCLOAK_TOOLS:-$HOME/Developer/Tools/mesail-keycloak}"
}

keycloak_home() {
  echo "$(keycloak_tools)/keycloak"
}

keycloak_wanted() {
  [ "${MESAIL_KEYCLOAK:-native}" = native ]
}

keycloak_port() {
  cat "$(run_folder)/keycloak-port"
}

keycloak_base() {
  echo "http://127.0.0.1:$(keycloak_port)"
}

keycloak_built() {
  [ -x "$(keycloak_home)/bin/kc.sh" ] && [ -f "$(keycloak_tools)/.built" ]
}

build_keycloak() {
  local staging
  if keycloak_built; then echo "native Keycloak is already built in $(keycloak_tools)"; return 0; fi
  staging="$(keycloak_tools).building-$$"
  rm -rf "$staging"
  "$(mesail_root)/deploy/keycloak/dev/build-native-keycloak.sh" "$staging"
  touch "$staging/.built"
  rm -rf "$(keycloak_tools)"
  mv "$staging" "$(keycloak_tools)"
}

prepare_keycloak_import() {
  local folder
  folder="$(run_folder)/keycloak-realm"
  rm -rf "$folder"
  cp -R "$(mesail_root)/deploy/keycloak/realm" "$folder"
  cp "$(mesail_root)"/deploy/floci/keycloak-users/*.json "$folder/dev/"
  echo "$folder"
}

start_keycloak() {
  local import management
  import="$(prepare_keycloak_import)"
  free_port > "$(run_folder)/keycloak-port"
  management="$(free_port)"
  mesail_load_secrets
  mkdir -p "$(data_folder)/keycloak"
  (KEYCLOAK_HOME="$(keycloak_home)" MESAIL_KEYCLOAK_HOME="$(mesail_root)/deploy/keycloak" \
    MESAIL_KEYCLOAK_IMPORT_DIR="$import" MESAIL_KEYCLOAK_DATA="$(cd "$(data_folder)/keycloak" && pwd -P)" \
    MESAIL_KEYCLOAK_URL="$MESAIL_ISSUER_ORIGIN/idp" MESAIL_KEYCLOAK_DEV_USERS=true \
    MESAIL_KEYCLOAK_DOMAINS="$(mesail_stack '.keycloak.domains | join(",")')" \
    KC_HTTP_HOST=127.0.0.1 KC_HTTP_PORT="$(keycloak_port)" KC_HTTP_MANAGEMENT_PORT="$management" \
    KC_BOOTSTRAP_ADMIN_CLIENT_ID=mesail-bootstrap \
    exec "$(mesail_root)/deploy/keycloak/bin/mesail-keycloak-start") > "$(mesail_log_dir)/keycloak.log" 2>&1 &
  echo "$!" >> "$(run_folder)/pids"
  echo "Keycloak starting on $(keycloak_base)/idp (pid $!, log keycloak.log)"
}

keycloak_answers() {
  curl --fail --silent "$(keycloak_base)/idp/realms/$(mesail_stack '.keycloak.domains[0]')/.well-known/openid-configuration" > /dev/null
}

provision_keycloak() {
  mesail_load_secrets
  KEYCLOAK_HOME="$(keycloak_home)" MESAIL_KEYCLOAK_ADMIN_URL="$(keycloak_base)/idp" \
    KC_BOOTSTRAP_ADMIN_CLIENT_ID=mesail-bootstrap "$(mesail_root)/deploy/keycloak/bin/mesail-keycloak-provision"
}

native_keycloak_url() {
  if ! keycloak_wanted; then echo "http://127.0.0.1:8180/idp"; return; fi
  echo "$(keycloak_base)/idp"
}

tokens_file() {
  echo "${MESAIL_TOKENS_FILE:-$(atlas_image)/$MESAIL_OWNER/tokens.env}"
}

write_tokens() {
  local file
  if ! keycloak_wanted; then return 0; fi
  file="$(tokens_file)"
  (umask 077 && MESAIL_KEYCLOAK_URL="$(keycloak_base)/idp" "$(mesail_root)/scripts/floci/tokens.sh" > "$file.new")
  mv "$file.new" "$file"
  echo "tokens for $(grep -c '^export MESAIL_TOKEN_' "$file") users in $file (mode 600; they last 60 s, renew with --tokens)"
}

directory_call() {
  local token="$1" method="$2" path="$3" body="${4:-}"
  curl --silent --output /dev/null --write-out '%{http_code}' --request "$method" \
    --header "Authorization: Bearer $token" --header 'Content-Type: application/json' \
    ${body:+--data "$body"} "$(router_base)/api/v1/directory$path"
}

seed_domain() {
  local domain="$1"
  echo "add domain $domain: $(directory_call "$MESAIL_TOKEN_OPERATOR" POST /domains "{\"name\":\"$domain\"}")"
  echo "verify domain $domain: $(directory_call "$MESAIL_TOKEN_OPERATOR" POST "/domains/$domain/verify")"
}

seed_member() {
  local name="$1" token="MESAIL_TOKEN_$1"
  echo "member $name: $(directory_call "${!token}" GET /members)"
}

seed_directory() {
  local domain name
  if ! keycloak_wanted || ! grep -q '^directory ' "$(run_folder)/servers"; then return 0; fi
  # shellcheck disable=SC1090
  . "$(tokens_file)"
  for domain in $(mesail_stack '.hostedDomains[]'); do
    seed_domain "$domain"
  done
  # shellcheck disable=SC2016
  for name in $(mesail_stack '.hostedDomains as $hosted | .testUsers[] | select(.fixture and (.realm | IN($hosted[]))) | .token'); do
    seed_member "$name"
  done
}
