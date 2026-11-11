#!/bin/sh
set -eu

usage() {
  echo "usage: rewrite-config.sh <application.json> <issuer origin> <keycloak url> <service base> [dev dns resolver]" >&2
  echo "  service base: a URL, or 'port:<n>' to keep each <name>.localhost host and use port n" >&2
  exit 64
}

service_rule() {
  case "$1" in
    port:*) printf 's#(http://[a-z0-9-]+\\.localhost):81(0[1-9]|1[0-9])#\\1:%s#g;s#http://127\\.0\\.0\\.1:81(0[1-9]|1[0-9])#http://127.0.0.1:%s#g' "${1#port:}" "${1#port:}" ;;
    *) printf 's#http://[a-z0-9-]+\\.localhost:81(0[1-9]|1[0-9])#%s#g;s#http://127\\.0\\.0\\.1:81(0[1-9]|1[0-9])#%s#g' "$1" "$1" ;;
  esac
}

host_of() {
  echo "$1" | sed -E 's#^[a-z]+://([^/:]+).*#\1#'
}

rewrite_addresses() {
  file="$1" origin="$2" keycloak="$3" services="$4"
  sed -E -i.before \
    -e "s#\"issuer\": *\"[^\"]*/idp/realms/#\"issuer\": \"$origin/idp/realms/#" \
    -e "s#http://keycloak:8080/idp#$keycloak#g" \
    -e "s#http://127\\.0\\.0\\.1:8180/idp#$keycloak#g" \
    -e "$(service_rule "$services")" "$file"
  rm -f "$file.before"
}

allow_hosts() {
  file="$1"
  shift
  jq --args '.outgoing.hosts = ((.outgoing.hosts // []) + $ARGS.positional | unique)' "$@" < "$file" > "$file.new"
  mv "$file.new" "$file"
}

use_resolver() {
  file="$1" resolver="$2"
  if [ -z "$resolver" ]; then return 0; fi
  sed -E -i.before -e "s#\"dnsresolver\": *\"[^\"]*\"#\"dnsresolver\": \"$resolver\"#" "$file"
  rm -f "$file.before"
}

main() {
  if [ "$#" -lt 4 ] || [ "$#" -gt 5 ]; then usage; fi
  rewrite_addresses "$1" "$2" "$3" "$4"
  use_resolver "$1" "${5:-}"
  case "$4" in
    port:*) allow_hosts "$1" "$(host_of "$3")" ;;
    *) allow_hosts "$1" "$(host_of "$3")" "$(host_of "$4")" ;;
  esac
}

main "$@"
