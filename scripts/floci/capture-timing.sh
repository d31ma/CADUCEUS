#!/bin/bash
set -u

now_ms() {
  local now="$EPOCHREALTIME" fraction
  fraction="${now#*.}"
  echo $((${now%.*} * 1000 + 10#${fraction:0:3}))
}

answer_seconds() {
  echo 600
}

fylo_binary() {
  if [ -x /usr/local/lib/mesail/fylo.real ]; then echo /usr/local/lib/mesail/fylo.real; return; fi
  command -v fylo
}

created_list() {
  echo /tmp/capture-created.txt
}

allowed_path() {
  local path="$1" name="$2"
  [ "$path" = "/mnt/data/$name" ] || [[ "$path" == "/mnt/data/.$name.fylo-root-owner.lock"* ]]
}

delete_path() {
  local path="$1" name="$2"
  if ! allowed_path "$path" "$name"; then echo "refused to delete $path"; return; fi
  if [ ! -e "$path" ]; then return; fi
  rm -rf -- "$path"
  echo "deleted $path"
}

volume_report() {
  echo "scratch volume after cleanup: $(du -sb /mnt/data | cut -f1) bytes, $(find /mnt/data -mindepth 1 | wc -l) entries"
}

remove_copy() {
  local name="$1" path
  while IFS= read -r path; do
    delete_path "$path" "$name"
  done < <(sort -u "$(created_list)" 2> /dev/null)
  volume_report
}

top_level_paths() {
  local tar_file="$1"
  tar -tf "$tar_file" | cut -d/ -f1 | sort -u | sed 's|^|/mnt/data/|'
}

check_free() {
  local name="$1" path
  while IFS= read -r path; do
    if ! allowed_path "$path" "$name"; then echo "refused: the copy holds $path, not only the root and its owner files"; return 1; fi
    if [ -e "$path" ]; then echo "refused: $path already exists in the scratch volume (an earlier capture was not cleaned)"; return 1; fi
  done < "$(created_list)"
}

unpack() {
  local name="$1" tar_file=/tmp/capture-copy.tar started
  started="$(now_ms)"
  cat > "$tar_file"
  top_level_paths "$tar_file" > "$(created_list)"
  check_free "$name" || return 1
  tar -x -C /mnt/data -f "$tar_file"
  rm -f "$tar_file"
  echo "unpack ms=$(($(now_ms) - started)) root=/mnt/data/$name files=$(find "/mnt/data/$name" -type f | wc -l)"
}

start_fylo() {
  local name="$1"
  coproc FYLO { exec "$(fylo_binary)" exec --loop --root "/mnt/data/$name" --max-request-bytes 1048576 --max-response-bytes 8388608 2>> /tmp/fylo-capture-stderr.log; }
}

ask() {
  local request="$1" answer
  printf '%s\n' "$request" >&"${FYLO[1]}"
  if ! IFS= read -r -t "$(answer_seconds)" answer <&"${FYLO[0]}"; then echo "no-answer-within-$(answer_seconds)s"; return 1; fi
  jq -r '"ok=\(.ok) code=\(.error.code // .code // "none")"' <<< "$answer" 2> /dev/null || echo "answer=not-json"
}

timed_step() {
  local name="$1" request="$2" started summary
  started="$(now_ms)"
  summary="$(ask "$request")" || { echo "step $name ms=$(($(now_ms) - started)) $summary"; return 1; }
  echo "step $name ms=$(($(now_ms) - started)) $summary"
}

store_steps() {
  local steps="$1" name request
  while IFS=$'\t' read -r name request; do
    timed_step "$name" "$request" || return 1
  done < <(jq -r '.[] | "\(.name)\t\(.request | tojson)"' <<< "$steps")
}

stop_fylo() {
  local input="${FYLO[1]}"
  exec {input}>&-
  wait "$FYLO_PID" 2> /dev/null
  echo "fylo exit rc=$?"
}

time_fylo() {
  local name="$1" steps="$2" started
  started="$(now_ms)"
  start_fylo "$name"
  timed_step "start+handshake" '{"op":"handshake"}' || return 1
  echo "start+handshake total ms=$(($(now_ms) - started))"
  store_steps "$steps"
  stop_fylo
}

fylo_stderr() {
  echo "--- fylo stderr"
  cat /tmp/fylo-capture-stderr.log 2> /dev/null || true
  echo "--- end of fylo stderr"
}

main() {
  local name="$1" steps="$2"
  if [[ ! "$name" =~ ^[a-z][a-z-]{0,31}$ ]]; then echo "refused: root name $name"; return 1; fi
  : > "$(created_list)"
  trap 'remove_copy "$name"' EXIT
  echo "fylo $("$(fylo_binary)" --version 2>&1 | head -1), user $(id -u):$(id -g), FYLO_ variables: $(env | grep -c '^FYLO_')"
  unpack "$name" || return 1
  time_fylo "$name" "$steps" || echo "timing stopped early (see the line above)"
  fylo_stderr
}

main "$@"
