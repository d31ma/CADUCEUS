#!/usr/bin/env bash
# shellcheck shell=bash

mesail_default_go_cache() {
  if [ "$(uname -s)" = Darwin ]; then echo "$HOME/Library/Caches/go-build-mesail"; fi
}

mesail_use_local_go_cache() {
  local folder
  folder="${MESAIL_GOCACHE:-$(mesail_default_go_cache)}"
  if [ -z "$folder" ]; then return 0; fi
  mkdir -p "$folder"
  export GOCACHE="$folder"
}
