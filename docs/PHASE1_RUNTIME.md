# Phase 1 Go Runtime Contract

Status: implementation acceptance contract

Phase 1 introduces one production-shaped vertical slice without moving mail,
authentication, or FYLO ownership away from the legacy server. The Go binary
owns only runtime diagnostics, operational HTTP endpoints, and static frontend
delivery during this phase.

## Commands

`caduceus version [--output text|json]`

- succeeds without configuration;
- reports the binary name, release version, source commit, build timestamp, Go
  version, operating system, and architecture;
- uses `dev` and `unknown` rather than empty development metadata.

`caduceus doctor [--config FILE] [--output text|json]`

- loads and validates the same configuration and dependencies as `serve`;
- checks the frontend bundle, FYLO executable/version, FYLO root, schema
  directory, runtime lock, and secret presence without starting FYLO or opening
  a listener;
- never prints secret values.

`caduceus serve [--config FILE]`

- completes strict configuration and dependency preflight before binding;
- serves the bundled login shell and SPA routes;
- exposes `/health`, `/healthz`, `/readyz`, `/system`, and
  `/api/v1/system`;
- exposes Prometheus text metrics only on the configured loopback operations
  listener;
- attaches or creates a bounded `X-Request-ID`, emits structured JSON request
  logs, and returns stable problem documents for API failures;
- marks readiness false before bounded graceful shutdown.

`caduceus migrate [--config FILE] [--output text|json]`

- performs a read-only Phase 1 migration preflight;
- reports the discovered schema count and runtime version;
- does not start FYLO or mutate production state. Authoritative FYLO migrations
  begin only after the Phase 2 single-writer cutover.

## Configuration

The canonical keys remain the uppercase names in
`contracts/config/v1.schema.json`. Loading order is defaults, optional JSON
file, environment, and non-secret command flags. JSON files reject unknown
keys. Known environment values are validated with the same rules.

Safe defaults bind public HTTP to `127.0.0.1:8080`, operations to
`127.0.0.1:9090`, use `build/static` for the synchronized Tachyon bundle, and
allow ten seconds for shutdown. Operations may not bind a non-loopback address.

## Acceptance Scenarios

1. A build with no injected metadata returns stable development metadata.
2. An unknown configuration key exits with usage failure and never owns the
   requested port.
3. A missing, non-executable, or version-incompatible FYLO dependency fails
   `doctor`, `serve`, and `migrate` before listener startup.
4. A valid runtime serves the exact synchronized login shell, a versioned
   system document, liveness, readiness, and loopback metrics.
5. Every response has security headers and a request ID; API errors use the
   Phase 0 problem shape.
6. `SIGINT` or `SIGTERM` makes readiness false, drains both listeners, and
   exits within the configured bound.
7. The legacy Phase 0 suite remains green; Phase 1 does not claim ownership of
   mail, authentication, or persistence.
