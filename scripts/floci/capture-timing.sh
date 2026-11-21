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

unpack_folder() {
  echo "/mnt/data/.$1-unpack"
}

remove_scratch() {
  local scratch="$1"
  rm -rf "/mnt/data/${scratch:?}" "$(unpack_folder "$scratch")"
  rm -f "/mnt/data/.$scratch.fylo-root-owner.lock"*
  echo "scratch root /mnt/data/$scratch removed"
}

rename_owner_file() {
  local file="$1" name="$2" scratch="$3" suffix
  suffix="${file##*/."$name"}"
  mv "$file" "/mnt/data/.$scratch$suffix"
}

move_owner_files() {
  local folder="$1" name="$2" scratch="$3" file
  for file in "$folder/.$name.fylo-root-owner.lock"*; do
    if [ -e "$file" ]; then rename_owner_file "$file" "$name" "$scratch"; fi
  done
}

unpack() {
  local name="$1" scratch="$2" folder started
  folder="$(unpack_folder "$scratch")"
  started="$(now_ms)"
  mkdir "$folder"
  tar -x -C "$folder"
  mv "$folder/$name" "/mnt/data/$scratch"
  move_owner_files "$folder" "$name" "$scratch"
  rmdir "$folder"
  echo "unpack ms=$(($(now_ms) - started)) root=/mnt/data/$scratch files=$(find "/mnt/data/$scratch" -type f | wc -l)"
}

start_fylo() {
  local scratch="$1"
  coproc FYLO { exec "$(fylo_binary)" exec --loop --root "/mnt/data/$scratch" --max-request-bytes 1048576 --max-response-bytes 8388608 2>> /tmp/fylo-capture-stderr.log; }
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
  local scratch="$1" steps="$2" started
  started="$(now_ms)"
  start_fylo "$scratch"
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
  local name="$1" scratch="$2" steps="$3"
  if [[ ! "$scratch" =~ ^capture-[0-9]{6}$ ]]; then echo "refused: scratch root name $scratch"; return 1; fi
  trap 'remove_scratch "$scratch"' EXIT
  echo "fylo $("$(fylo_binary)" --version 2>&1 | head -1), user $(id -u):$(id -g), FYLO_ variables: $(env | grep -c '^FYLO_')"
  unpack "$name" "$scratch" || return 1
  time_fylo "$scratch" "$steps" || echo "timing stopped early (see the line above)"
  fylo_stderr
}

main "$@"
