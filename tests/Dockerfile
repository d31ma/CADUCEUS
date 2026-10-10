FROM oven/bun:1.3.14@sha256:e10577f0db68676a7024391c6e5cb4b879ebd17188ab750cf10024a6d700e5c4

WORKDIR /app

# Match the checksum-verified standalone binaries used by the production image.
ARG TACHYON_VERSION=26.33.02
ARG FYLO_VERSION=26.33.02
ARG CHEX_VERSION=26.32.02
ARG TTID_VERSION=26.32.03
RUN apt-get update \
  && apt-get install -y --no-install-recommends ca-certificates curl \
  && arch="$(uname -m)" \
  && case "$arch" in x86_64|amd64) release_arch=x64 ;; arm64|aarch64) release_arch=arm64 ;; *) exit 1 ;; esac \
  && install_release() { \
       repo="$1"; binary="$2"; version="$3"; asset="${binary}-linux-${release_arch}"; \
       base="https://github.com/d31ma/${repo}/releases/download/v${version}"; \
       tmp="$(mktemp -d)"; \
       curl -fsSL "${base}/${asset}" -o "${tmp}/${binary}"; \
       curl -fsSL "${base}/SHA256SUMS" -o "${tmp}/SHA256SUMS"; \
       expected="$(awk -v asset="$asset" '$2 == asset || $2 == "*" asset { print tolower($1) }' "${tmp}/SHA256SUMS")"; \
       actual="$(sha256sum "${tmp}/${binary}" | awk '{ print tolower($1) }')"; \
       test -n "$expected" && test "$expected" = "$actual"; \
       install -m 755 "${tmp}/${binary}" "/usr/local/bin/${binary}"; \
       rm -rf "$tmp"; \
     }; \
     install_release Tachyon ty "$TACHYON_VERSION"; \
     install_release Fylo fylo "$FYLO_VERSION"; \
     install_release Chex chex "$CHEX_VERSION"; \
     install_release TTID ttid "$TTID_VERSION" \
  && test "$(ty --version)" = "${TACHYON_VERSION}" \
  && rm -rf /var/lib/apt/lists/*

COPY package.json bun.lock ./
RUN bun install

COPY tests/ ./tests/
COPY server/ ./server/
COPY scripts/ ./scripts/
COPY jsconfig.json ./
COPY tachyon-env.d.ts ./

ENV NODE_ENV=test

CMD ["bun", "test", "tests/integration/", "--timeout", "60000"]
