# Phase 0 Baseline

Status: complete  
Captured: 2026-07-26  
Base branch: `main`  
Base commit: `02ee4770b5ff98ac9eb4cfebc279f85f0b190daf`

This document freezes the observable legacy behavior that the Go migration
must preserve. It does not claim that the current JavaScript implementation is
the target architecture.

This is historical Phase 0 evidence. The FYLO 26.30.04 and local-preview
constraints recorded below are superseded for current deployments by the
released FYLO 26.33.02 qualification in
[`PHASE2_EVIDENCE.md`](PHASE2_EVIDENCE.md).

## Reproduce

Use the locked Bun dependency graph and the binaries described by
[`runtime.lock.json`](../runtime.lock.json):

```sh
bun install --frozen-lockfile
bun run verify:phase0
bun run verify:phase0:browser
```

Cloud file providers must keep the source tree and `node_modules` available
offline during qualification. If they evict dependency or source files, run
the frozen install and verification in an ephemeral non-cloud copy; dataless
placeholders are an environment failure, not a valid test result.

For the unreleased FYLO access-control qualification:

```sh
FYLO_BINARY=../FYLO/dist-bin/fylo bun run test:fylo-local
```

The local preview is evidence for development only. It is not a supported
production dependency because its build identity reports an unknown commit and
the source worktree is dirty.

## Baseline Results

| Gate | Result | Evidence |
| --- | --- | --- |
| TypeScript validation | Passing after five FYLO boundary annotations were corrected | `bun run typecheck` |
| Legacy unit, runtime, persistence, route, and HTTP suites | 723 passed, 0 failed | `bun run test` |
| Tachyon frontend bundle | Passing, 34 web routes | `bun run bundle` |
| Route response-schema audit | Passing, 57 route modules | `bun run audit:schemas` |
| Complexity regression gate | 655 passed; 510 functions measured; 0 above CRAP 15; worst 15.0 | `bun run crap` |
| Local FYLO issue-68 bridge | Passing, document and bucket access denied across virtual groups | `FYLO_BINARY=../FYLO/dist-bin/fylo bun run test:fylo-local` |
| Browser interaction behavior | 31 passed, 0 failed | `bun run test:e2e` |
| Responsive visual behavior | 40 passed, 0 failed; 35 route snapshots across five viewport classes | `bun run test:matrix` |

There are no accepted failures in the Phase 0 gates. Tooling and environment
limitations that are not test failures are listed under Known Baseline
Constraints.

Any future baseline failure is a regression unless it is accepted through an
ADR and the contract fixtures are intentionally versioned.

## Runtime and Port Ownership

### Current migration baseline

| Listener or process | Owner | Development default | Container mapping |
| --- | --- | --- | --- |
| HTTP and static assets | Tachyon `ty serve` | `127.0.0.1:8080` | `8080:8080` |
| Inbound SMTP | JavaScript supervisor | disabled; `25` when enabled | `25:2525` for non-root container |
| SMTP submission STARTTLS | JavaScript supervisor | disabled; `587` when enabled | `587:2587` |
| SMTP submission implicit TLS | JavaScript supervisor | disabled; `465` when enabled | operator-defined |
| ACME HTTP-01 | JavaScript supervisor | disabled; `80` when enabled | `80:80` |
| FYLO | one child process per application database owner | `${FYLO_ROOT}` | `/data/caduceus` |

### Target ownership

The production Go process owns HTTP, static assets, SMTP, submission, delivery
workers, authentication, authorization, migrations, and the one persistent
FYLO child process. Tachyon remains a frontend build and development tool. A
deployment must never run JavaScript and Go as concurrent FYLO writers.

## Artifact Model

- Production artifact: one Linux OCI image for `linux/amd64` and `linux/arm64`.
- Release unit: immutable image digest, not a mutable tag.
- Runtime dependencies: exact released binaries and digests from
  `runtime.lock.json`.
- Data unit: FYLO root plus bucket data, schemas, access metadata, and the
  release manifest required to restore it.
- Frontend assets: generated once by Tachyon and embedded in the production
  image served by Go.

## Support Vocabulary

| Term | Meaning |
| --- | --- |
| `buildable` | The artifact compiles or bundles on the named target. |
| `native-tested` | The named artifact passed its test suite on that native target. |
| `preview` | Useful for development and qualification, but not approved for production data. |
| `supported` | Build, native tests, security gates, upgrade, restore, rollback, and operational evidence all passed for the named topology. |

Cross-compilation is only `buildable`. A local dirty binary is only `preview`.
No document or release may silently promote either status to `supported`.

## Frozen Behavior

The compatibility corpus is
[`contracts/parity/v1.json`](../contracts/parity/v1.json). It maps every
migration phase to tests that observe the system through HTTP, SMTP, browser,
or FYLO process boundaries. The canonical target routes are
[`contracts/http/routes.v1.json`](../contracts/http/routes.v1.json).

The current Tachyon server exposes unversioned compatibility paths. Go will
introduce `/api/v1` while retaining those aliases through Phase 7. Test
expectations remain unchanged; only the base URL and prefix adapter may vary.

## Known Baseline Constraints

- The installed released FYLO `26.30.04` does not advertise the required
  handshake or exclusive-root capabilities.
- The sibling FYLO preview implements and passes the access-control bridge test,
  but is not reproducible release material.
- The current runtime is a single-host, single-writer topology. Shared network
  filesystems and active-active writers are unsupported.
- The current API uses legacy error marker objects. The stable v1 problem
  contract is defined now and implemented during the Go slices.
- Test-only routes are allowed only when both `NODE_ENV=test` and
  `CADUCEUS_ENABLE_TEST_ROUTES=true`; they are removed from production images.

## Checkpoint

`bun run checkpoint:phase0` creates a non-destructive local recovery package in
`.phase0-checkpoints/`. It contains:

- a complete archive of every existing tracked and untracked non-ignored file;
- an explicit list of tracked paths deleted relative to the base commit;
- the exact base commit, tracked/untracked inventory, file lists, and SHA-256
  digests.

The directory is intentionally ignored because it can contain a complete copy
of uncommitted work. It must be stored with the same access controls as the
source repository and must never be uploaded as a CI artifact.
