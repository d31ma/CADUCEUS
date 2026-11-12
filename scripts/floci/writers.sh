#!/usr/bin/env bash
set -euo pipefail
# shellcheck source-path=SCRIPTDIR
. "$(dirname "$0")/common.sh"

container_prefix() {
  echo "floci-aws-$(mesail_stack .functionPrefix)$1-"
}

fylo_functions() {
  mesail_stack '.services[] | select((.kind // "lambda") == "lambda" and .fylo) | .name'
}

containers_oldest_first() {
  docker ps --filter "name=$(container_prefix "$1")" --format '{{.CreatedAt}}|{{.Names}}' | sort | cut -d'|' -f2
}

extra_containers() {
  containers_oldest_first "$1" | sed '$d'
}

stop_container() {
  local service="$1" name="$2"
  echo "$(date -u +%Y-%m-%dT%H:%M:%SZ) writers: $service has more than one container; stopping the older $name"
  docker stop --time 10 "$name" > /dev/null
}

stop_extra_containers() {
  local service="$1" name
  for name in $(extra_containers "$service"); do
    stop_container "$service" "$name"
  done
}

check_once() {
  local service
  for service in $(fylo_functions); do
    stop_extra_containers "$service"
  done
}

stop_all_containers() {
  local service="$1" name
  for name in $(containers_oldest_first "$service"); do
    echo "$(date -u +%Y-%m-%dT%H:%M:%SZ) writers: stopping $name of $service before its redeploy"
    docker stop --time 10 "$name" > /dev/null
  done
}

watch() {
  local seconds="$1"
  echo "$(date -u +%Y-%m-%dT%H:%M:%SZ) writers: checking every $seconds s for a second container of a FYLO function"
  while true; do
    check_once
    sleep "$seconds"
  done
}

main() {
  mesail_load_settings
  case "${1:-}" in
    watch) watch "${2:-5}" ;;
    check) check_once ;;
    before-redeploy) stop_all_containers "$2" ;;
    *) echo "usage: writers.sh watch [seconds] | check | before-redeploy <service>"; return 1 ;;
  esac
}

main "$@"
