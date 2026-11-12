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

rebuild_wanted() {
  [ "${MESAIL_KEYCLOAK_REBUILD:-0}" = 1 ]
}

keycloak_reusable() {
  keycloak_built && ! rebuild_wanted
}

keycloak_inputs_hash() {
  (cd "$(mesail_root)/deploy/keycloak" && find dev providers conf -type f -print0 | LC_ALL=C sort -z | xargs -0 shasum -a 256) \
    | shasum -a 256 | cut -d' ' -f1
}

keycloak_stamp() {
  cat "$(keycloak_tools)/.built"
}

warn_stale_keycloak() {
  if [ "$(keycloak_stamp)" = "$(keycloak_inputs_hash)" ]; then return 0; fi
  echo "warning: the shared native Keycloak was not built from this checkout's deploy/keycloak (dev, providers, conf)."
  echo "  Rebuild it when no other native run uses it: MESAIL_KEYCLOAK_REBUILD=1 scripts/native/run-all.sh"
}

reuse_keycloak() {
  echo "native Keycloak is already built in $(keycloak_tools)"
  warn_stale_keycloak
}

keycloak_in_use() {
  pgrep -f "$(keycloak_home)/bin/" > /dev/null
}

refuse_busy_rebuild() {
  if ! keycloak_in_use; then return 0; fi
  echo "cannot rebuild: a running Keycloak (another native run) uses $(keycloak_tools); wait until that run ends"
  return 1
}

reuse_download() {
  local staging="$1"
  mkdir -p "$staging"
  if [ ! -d "$(keycloak_tools)" ]; then return 0; fi
  find "$(keycloak_tools)" -maxdepth 1 -name 'keycloak-*.tar.gz' -exec cp {} "$staging/" \;
}

build_keycloak() {
  local staging
  if keycloak_reusable; then reuse_keycloak; return 0; fi
  refuse_busy_rebuild || return 1
  staging="$(keycloak_tools).building-$$"
  rm -rf "$staging"
  reuse_download "$staging"
  "$(mesail_root)/deploy/keycloak/dev/build-native-keycloak.sh" "$staging"
  keycloak_inputs_hash > "$staging/.built"
  rm -rf "$(keycloak_tools)"
  mv "$staging" "$(keycloak_tools)"
}

prepare_keycloak_import() {
  local folder
  folder="$(run_folder)/keycloak-realm"
  rm -rf "$folder"
  cp -R "$(mesail_root)/deploy/keycloak/realm" "$folder"
  echo "$folder"
}

start_keycloak() {
  local import management
  free_port > "$(run_folder)/keycloak-port"
  management="$(free_port)"
  import="$(prepare_keycloak_import)"
  mesail_load_secrets
  mkdir -p "$(data_folder)/keycloak"
  (KEYCLOAK_HOME="$(keycloak_home)" MESAIL_KEYCLOAK_HOME="$(mesail_root)/deploy/keycloak" \
    MESAIL_KEYCLOAK_IMPORT_DIR="$import" MESAIL_KEYCLOAK_DATA="$(cd "$(data_folder)/keycloak" && pwd -P)" \
    MESAIL_KEYCLOAK_URL="$(keycloak_base)/idp" MESAIL_KEYCLOAK_DEV_USERS=true \
    KC_HTTP_HOST=127.0.0.1 KC_HTTP_PORT="$(keycloak_port)" KC_HTTP_MANAGEMENT_PORT="$management" \
    exec "$(mesail_root)/deploy/keycloak/bin/mesail-keycloak-start") > "$(mesail_log_dir)/keycloak.log" 2>&1 &
  echo "$!" >> "$(run_folder)/pids"
  echo "Keycloak starting on $(keycloak_base)/idp (pid $!, log keycloak.log)"
}

keycloak_answers() {
  curl --fail --silent "$(keycloak_base)/idp/realms/$(mesail_stack .keycloak.realm)/.well-known/openid-configuration" > /dev/null
}

native_keycloak_url() {
  if ! keycloak_wanted; then echo "http://127.0.0.1:8180/idp"; return; fi
  echo "$(keycloak_base)/idp"
}

tokens_file() {
  echo "${MESAIL_TOKENS_FILE:-$(atlas_image)/$MESAIL_OWNER/tokens.env}"
}

sign_in_fixtures() {
  local file="$1"
  if (umask 077 && MESAIL_ROUTER_URL="$(router_base)" "$(mesail_root)/scripts/floci/tokens.sh" > "$file"); then return 0; fi
  rm -f "$file"
  echo "could not sign in the fixture people; the old tokens file stays"
  return 1
}

write_tokens() {
  local file
  if ! keycloak_wanted; then return 0; fi
  file="$(tokens_file)"
  sign_in_fixtures "$file.new" || return 1
  mv "$file.new" "$file"
  echo "tokens for $(grep -c '^export MESAIL_TOKEN_[A-Z]*=[A-Za-z0-9]' "$file") of $(grep -c '^export MESAIL_TOKEN_' "$file") people in $file (mode 600; they last 60 s, renew with --tokens)"
}

seed_services_run() {
  grep -q '^identity ' "$(run_folder)/servers" && grep -q '^directory ' "$(run_folder)/servers"
}

seed_directory() {
  if ! keycloak_wanted || ! seed_services_run; then echo "no seed: needs Keycloak, identity and directory"; return 0; fi
  mesail_load_secrets
  mesail_seed_directory "$(router_base)"
}
