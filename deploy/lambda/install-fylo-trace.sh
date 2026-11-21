#!/bin/sh
set -eu

real_fylo() {
  echo /usr/local/lib/mesail/fylo.real
}

install_wrapper() {
  mv /usr/local/bin/fylo "$(real_fylo)"
  install -m 755 /usr/local/lib/mesail/fylo-trace.sh /usr/local/bin/fylo
  echo "fylo trace wrapper installed (MESAIL_FYLO_TRACE=1, test image only)"
}

main() {
  if [ "$1" != 1 ]; then echo "fylo trace wrapper not installed (MESAIL_FYLO_TRACE=$1)"; return 0; fi
  install_wrapper
}

main "$@"
