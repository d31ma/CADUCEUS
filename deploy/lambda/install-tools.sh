#!/bin/sh
set -eu

release_name() {
  case "$1" in
    arm64) echo arm64 ;;
    amd64) echo x64 ;;
    *) echo "unsupported architecture: $1" >&2; return 1 ;;
  esac
}

checksum_for() {
  case "$1" in
    arm64) echo "$2" ;;
    amd64) echo "$3" ;;
    *) echo "unsupported architecture: $1" >&2; return 1 ;;
  esac
}

download_checked() {
  url="$1" expected="$2" target="$3"
  echo "=== download $url"
  curl --fail --silent --show-error --location --retry 3 "$url" --output "$target"
  echo "$expected  $target" | sha256sum --check --strict
}

install_go() {
  architecture="$1"
  expected="$(checksum_for "$architecture" "$GO_SHA256_ARM64" "$GO_SHA256_AMD64")"
  download_checked "https://go.dev/dl/go${GO_VERSION}.linux-${architecture}.tar.gz" "$expected" /tmp/go.tar.gz
  tar -C /usr/local -xzf /tmp/go.tar.gz
  rm /tmp/go.tar.gz
}

install_tachyon() {
  architecture="$1"
  expected="$(checksum_for "$architecture" "$TY_SHA256_ARM64" "$TY_SHA256_AMD64")"
  download_checked "https://github.com/d31ma/TACHYON/releases/download/v${TACHYON_VERSION}/ty-linux-$(release_name "$architecture")" "$expected" /tmp/ty
  install -m 755 /tmp/ty /usr/local/bin/ty
  rm /tmp/ty
}

install_fylo() {
  architecture="$1"
  expected="$(checksum_for "$architecture" "$FYLO_SHA256_ARM64" "$FYLO_SHA256_AMD64")"
  download_checked "https://github.com/d31ma/Fylo/releases/download/v${FYLO_VERSION}/fylo-linux-$(release_name "$architecture")" "$expected" /tmp/fylo
  install -m 755 /tmp/fylo /usr/local/bin/fylo
  rm /tmp/fylo
}

check_versions() {
  /usr/local/go/bin/go version | grep -F "go${GO_VERSION} "
  test "$(ty --version)" = "$TACHYON_VERSION"
  fylo --version | grep -F "$FYLO_VERSION"
}

main() {
  architecture="$(dpkg --print-architecture)"
  install_go "$architecture"
  install_tachyon "$architecture"
  install_fylo "$architecture"
  check_versions
}

main "$@"
