# syntax=docker/dockerfile:1.7

FROM golang:1.26.5-bookworm@sha256:6c5605ab3a9a9fb3c4eafe5b3d63cdbf3881caf113262b67862547b54a9db599 AS go-builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -buildvcs=false -trimpath -ldflags="-s -w" -o /out/caduceus-go ./cmd/caduceus

# ── Build stage: install dependencies and strip test routes ──────────────────
FROM oven/bun:1.3.14@sha256:e10577f0db68676a7024391c6e5cb4b879ebd17188ab750cf10024a6d700e5c4 AS builder

WORKDIR /app

# Install the exact standalone binary versions used by this project. Each
# download is verified against its release SHA256SUMS before installation.
ARG TACHYON_VERSION=26.33.02
ARG FYLO_VERSION=26.33.02
ARG SESAME_VERSION=26.33.02
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
     install_release TTID ttid "$TTID_VERSION"; \
     case "$arch" in x86_64|amd64) go_arch=amd64 ;; arm64|aarch64) go_arch=arm64 ;; esac; \
     sesame_asset="sesame-linux-${go_arch}"; \
     sesame_base="https://github.com/d31ma/SESAME/releases/download/v${SESAME_VERSION}"; \
     sesame_tmp="$(mktemp -d)"; \
     curl -fsSL "${sesame_base}/${sesame_asset}" -o "${sesame_tmp}/sesame"; \
     curl -fsSL "${sesame_base}/SHA256SUMS" -o "${sesame_tmp}/SHA256SUMS"; \
     sesame_expected="$(awk -v asset="$sesame_asset" '$2 == asset || $2 == "*" asset { print tolower($1) }' "${sesame_tmp}/SHA256SUMS")"; \
     sesame_actual="$(sha256sum "${sesame_tmp}/sesame" | awk '{ print tolower($1) }')"; \
     test -n "$sesame_expected" && test "$sesame_expected" = "$sesame_actual"; \
     install -m 755 "${sesame_tmp}/sesame" /usr/local/bin/sesame; \
     rm -rf "$sesame_tmp" \
  && rm -rf /var/lib/apt/lists/*

COPY package.json bun.lock ./
COPY translation-models.lock.json ./translation-models.lock.json
# Tachyon compiles checked JavaScript with the locked TypeScript toolchain at
# startup, so the compiler is a runtime build dependency during migration.
RUN bun install --frozen-lockfile

COPY client ./client
COPY server ./server
COPY background ./background
COPY db/schemas ./db/schemas
COPY scripts/entry.mjs ./scripts/
COPY scripts/create-admin.mjs ./scripts/
COPY scripts/migrate-domain.mjs ./scripts/
COPY scripts/vendor-translation-assets.mjs ./scripts/
COPY scripts/vendor-server-deps.mjs ./scripts/
COPY docs/THIRD_PARTY_MODELS.md ./docs/THIRD_PARTY_MODELS.md
COPY jsconfig.json ./jsconfig.json
COPY runtime.lock.json ./runtime.lock.json

# Translation is optional at use time but completely self-hosted. Resolve the
# immutable model revision during the image build, verify every byte against
# translation-models.lock.json, and prohibit runtime network model fetches.
RUN bun run translation:vendor && bun run translation:verify

# Tachyon runs handlers from a snapshot of server/ under Node, so the npm
# packages they import are bundled into server/deps first.
RUN bun scripts/vendor-server-deps.mjs

# Remove test routes — they should never be in production
RUN rm -f server/controllers/Test*Controller.js \
  && ! ls server/controllers | grep -q '^Test.*Controller\.js$'

# Validate the image can actually serve — catch missing deps / broken bundles
# at build time rather than deploying a broken container that fails at runtime.
RUN command -v ty \
  && command -v fylo \
  && command -v sesame \
  && command -v chex \
  && command -v ttid \
  && test "$(ty --version)" = "${TACHYON_VERSION}" \
  && test "$(fylo --version)" = "${FYLO_VERSION}" \
  && sesame version | grep -F "sesame ${SESAME_VERSION} " \
  && test -f scripts/entry.mjs \
  && test -f scripts/create-admin.mjs \
  && test -f scripts/migrate-domain.mjs \
  && test -f translation-assets/catalog.json \
  && test -f docs/THIRD_PARTY_MODELS.md

# Pre-create data directory structure (distroless has no shell for RUN)
# The engine's deployment is a sibling of the mail root, never inside it: one
# FYLO process owns one root, and these are two.
RUN mkdir -p /data/caduceus /data/caduceus/.caduceus-incoming /data/sesame \
  && chown -R 65532:65532 /data \
  && mkdir -p /home/bun/.bun \
  && cp -a /root/.bun/. /home/bun/.bun/ \
  && chown -R 65532:65532 /home/bun

# ── Runtime: distroless Bun — minimal attack surface ─────────────────────────
FROM oven/bun:1.3.14-distroless@sha256:c28c51287af70bab8e0b66fc4b6a30cfb92a727ebc88045223adc9f4c9d09307

COPY --from=builder --chown=65532:65532 /app /app
# The home directory is prepared in the builder rather than left to an
# implicitly created COPY parent: BuildKit chowns those, the classic builder
# leaves them root-owned, and the frontend bundler needs to write its cache
# there. Preparing it explicitly makes the image the same either way.
COPY --from=builder --chown=65532:65532 /home/bun /home/bun
COPY --from=builder --chown=65532:65532 /data /data
COPY --from=builder /usr/local/bin/ty /usr/local/bin/ty
COPY --from=builder /usr/local/bin/fylo /usr/local/bin/fylo
COPY --from=builder /usr/local/bin/sesame /usr/local/bin/sesame
COPY --from=builder /usr/local/bin/chex /usr/local/bin/chex
COPY --from=builder /usr/local/bin/ttid /usr/local/bin/ttid
COPY --from=go-builder /out/caduceus-go /usr/local/bin/caduceus-go

WORKDIR /app
ENV HOME=/home/bun
USER 65532:65532

ENV NODE_ENV=production \
    HOST=0.0.0.0 \
    PORT=8080 \
    FYLO_ROOT=/data/caduceus \
    FYLO_BINARY=/usr/local/bin/fylo \
    FYLO_SCHEMA=/app/db/schemas \
    FYLO_INCOMING_ROOT=/data/caduceus/.caduceus-incoming \
    FYLO_BRIDGE_ADDRESS=127.0.0.1:9876 \
    FYLO_POSIX_ACCESS=true \
    SESAME_BINARY=/usr/local/bin/sesame \
    SESAME_DEPLOYMENT=/data/sesame \
    TRANSLATION_ASSET_ROOT=/app/translation-assets \
    CADUCEUS_GO_BINARY=/usr/local/bin/caduceus-go \
    RUNTIME_LOCK=/app/runtime.lock.json \
    FYLO_INDEX_BACKEND=local-fs \
    SMTP_ADAPTER=direct

VOLUME ["/data"]
# 8080 = HTTP (web + API). 25 = inbound SMTP (MAIL_SERVER_ENABLED=true).
# 587/465 = authenticated submission (MAIL_SUBMISSION_ENABLED=true). 80 = ACME
# HTTP-01 challenge (MAIL_ACME_ENABLED=true). The host must also allow
# inbound/outbound TCP 25 for a true standalone MTA.
EXPOSE 8080 25 587 465 80

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s \
  CMD ["/usr/local/bin/bun","-e","const r=await fetch('http://127.0.0.1:'+(process.env.PORT||'8080')+'/health');process.exit(r.ok?0:1)"]

ENTRYPOINT ["/usr/local/bin/bun", "scripts/entry.mjs"]
CMD []
