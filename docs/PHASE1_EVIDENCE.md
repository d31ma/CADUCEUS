# Phase 1 Qualification Evidence

Status: complete  
Qualification date: 2026-07-26

## Scope

This record qualifies the Phase 1 Go runtime foundation defined by
`docs/PHASE1_RUNTIME.md`. It does not authorize the Go runtime to own FYLO,
SMTP, submission, delivery workers, authentication, or business APIs.

## Acceptance Mapping

| Exit criterion | Evidence |
| --- | --- |
| Go serves the bundled login shell and system endpoint | Black-box tests build the real binary, serve the synchronized Tachyon bundle, fetch the login shell, and validate `/api/v1/system` against its stable response shape. |
| Invalid configuration and missing dependencies fail before listeners open | Configuration, preflight, and black-box tests cover unknown keys, weak secrets, missing artifacts, invalid manifests, and occupied-port probes before serving. |
| No production behavior silently changed | The production Docker and legacy runtime entrypoints remain unchanged; Phase 1 is documented as a code-only rollback class and `caduceus migrate` is read-only with `mutated=false`. |

## Qualification Gates

The completion gate is:

```sh
bun run verify:phase1:all
```

It composes the frozen Phase 0 application/browser qualification with:

- formatting, unit, black-box, race, and vet checks for Go;
- an 80% minimum Go coverage floor and a maximum CRAP score of 15;
- static analysis and vulnerability scanning in CI;
- a reproducible metadata-injected Go build;
- Tachyon bundle synchronization and manifest integrity verification;
- runtime contract, configuration, route, and toolchain checks.

Measured results:

| Gate | Result |
| --- | --- |
| Strict TypeScript | Passed both TypeScript configurations. |
| Response contracts | 57 routes scanned; every emittable status declared. |
| Core application suite | 723 passed across 40 files. |
| JavaScript CRAP gate | 510 production functions; 0 above 15; worst 15.0. |
| Browser interactions | 31 passed. |
| Responsive visual matrix | 40 passed across phone, tablet, landscape tablet, desktop, and wide viewports. |
| Tachyon bundle | 34 routes and 42 synchronized assets; digest `fb7f8487b1e8a5c17a265613ae15ed5223e8e1cf4abdbc920121ee0177b057fe`. |
| Go tests | Unit, HTTP, preflight, CLI, black-box, and race suites passed. |
| Go coverage | 84.4% statements against an 80% floor. |
| Go CRAP gate | 79 production functions; 0 above 15; worst 13.1. |
| Go static analysis | `go vet`, staticcheck 0.7.0, and govulncheck 1.6.0 passed; no known vulnerabilities found. |
| CI workflow | YAML parsing and actionlint 1.7.7 passed. |

The full `bun run verify:phase1:all` command completed successfully from the
non-cloud staging copy on 2026-07-26. The staging copy was used to remove
Dropbox hydration latency from measurements; the source changes were
synchronized from this workspace immediately before the run.

## Runtime Observations

- The Go runtime reports build identity and Go toolchain metadata without
  exposing configuration secrets.
- The public and loopback operations listeners are acquired before serving.
- Readiness is withdrawn before graceful shutdown and both listeners close
  within the configured bound.
- The released FYLO artifact is identified by the locked platform digest.
  Phase 1 does not start it.
- Only files listed in the verified frontend manifest can be served.

## Rollback

Phase 1 is code-only. Stop the Go process and restart the previous approved
application digest. No FYLO restore is required because Phase 1 has no
persistent writer.
