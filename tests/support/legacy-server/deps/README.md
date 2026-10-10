# Binary shims

Dependency-free Node shims for the standalone binaries installed with Tachyon:

- `fylo.mjs` — FYLO 26.33.02, with CADUCEUS bridge authentication, UID/GID
  access descriptors, paginated queries, bucket reads, the queue surface, and
  the 26.33.02 explicit child-environment/`.env` option
- `chex.mjs` — CHEX 26.32.02
- `ttid.mjs` — TTID 26.32.03, including canonical TTID normalization

Each shim starts its binary in persistent NDJSON loop mode. Instantiate it once
per application lifetime and call `close()` during controlled shutdown.

`FYLO_BINARY` may point at an explicitly qualified executable. Production
preflight still requires its digest and identity to match `runtime.lock.json`.
