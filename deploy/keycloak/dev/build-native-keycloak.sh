#!/usr/bin/env bash
set -Eeuo pipefail

readonly KEYCLOAK_VERSION=26.8.2
readonly KEYCLOAK_SHA1=7918de2786c7db466152128bea350becbd40a58f
readonly KEYCLOAK_URL=https://github.com/keycloak/keycloak/releases/download/${KEYCLOAK_VERSION}/keycloak-${KEYCLOAK_VERSION}.tar.gz
readonly SQLITE_JDBC_VERSION=3.54.1.0
readonly SQLITE_JDBC_SHA256=f8dce420a6016278497f4f0560f65602b9db858de141b541cf91a4644ebe4639
readonly SQLITE_JDBC_URL=https://repo1.maven.org/maven2/org/xerial/sqlite-jdbc/${SQLITE_JDBC_VERSION}/sqlite-jdbc-${SQLITE_JDBC_VERSION}.jar
REPO_KEYCLOAK=$(cd "$(dirname "$0")/.." && pwd)
readonly REPO_KEYCLOAK

step() {
  printf '=== %s %s\n' "$1" "$(date +%H:%M:%S)"
}

usage() {
  echo "usage: build-native-keycloak.sh <folder> [sqlite|postgres]" >&2
  exit 2
}

download_checked() {
  local url=$1 file=$2 algorithm=$3 expected=$4 actual
  [[ -f $file ]] || curl -fsSL -o "$file" "$url"
  actual=$(shasum -a "$algorithm" "$file" | awk '{print $1}')
  [[ $actual == "$expected" ]] || { echo "checksum mismatch for $file" >&2; rm -f "$file"; exit 1; }
}

unpack_keycloak() {
  local folder=$1
  step "download and unpack Keycloak ${KEYCLOAK_VERSION}"
  download_checked "$KEYCLOAK_URL" "$folder/keycloak-${KEYCLOAK_VERSION}.tar.gz" 1 "$KEYCLOAK_SHA1"
  rm -rf "$folder/keycloak"
  mkdir -p "$folder/keycloak"
  tar -xzf "$folder/keycloak-${KEYCLOAK_VERSION}.tar.gz" -C "$folder/keycloak" --strip-components 1
}

build_provider() {
  local folder=$1 provider=$2
  local classes=$folder/provider-classes/$provider source sources=()
  while IFS= read -r source; do
    sources+=("$source")
  done < <(find "$REPO_KEYCLOAK/providers/$provider/src/main/java" -name '*.java')
  step "compile the ${provider} provider"
  rm -rf "$classes"
  javac --release 21 -Xlint:all -Werror -cp "$folder/keycloak/lib/lib/main/*:$folder/keycloak/lib/lib/boot/*" -d "$classes" "${sources[@]}"
  cp -R "$REPO_KEYCLOAK/providers/$provider/src/main/resources/." "$classes/"
  jar --create --file "$folder/keycloak/providers/mesail-keycloak-${provider}.jar" -C "$classes" .
}

build_sqlite_providers() {
  build_provider "$1" sqlite
  download_checked "$SQLITE_JDBC_URL" "$1/keycloak/providers/sqlite-jdbc.jar" 256 "$SQLITE_JDBC_SHA256"
}

configure() {
  local folder=$1 database=$2
  step "kc.sh build (${database})"
  cp "$REPO_KEYCLOAK/conf/keycloak-${database}.conf" "$folder/keycloak/conf/keycloak.conf"
  cp "$REPO_KEYCLOAK/conf/quarkus.properties" "$folder/keycloak/conf/quarkus.properties"
  "$folder/keycloak/bin/kc.sh" build
}

main() {
  [[ $# -ge 1 ]] || usage
  local folder=$1 database=${2:-sqlite}
  mkdir -p "$folder"
  folder=$(cd "$folder" && pwd)
  unpack_keycloak "$folder"
  build_provider "$folder" mesail
  if [[ $database == sqlite ]]; then
    build_sqlite_providers "$folder"
  fi
  configure "$folder" "$database"
  step "done: KEYCLOAK_HOME=$folder/keycloak"
}

main "$@"
