# Phase 2 FYLO Viability and Single-Writer Contract

Status: complete  
Date: 2026-07-26

## Objective

Prove that the released FYLO machine interface is safe enough to become the
single persistence process for CADUCEUS, then route the legacy JavaScript
runtime through that Go-owned process without changing public mail behavior.

Phase 2 changes persistence ownership. It does not migrate authentication,
mailbox business logic, SMTP, submission, or delivery workers to Go.

## Process Boundary

```text
legacy Tachyon routes -- authenticated loopback bridge --+
                                                         |
Go feature services -------------------------------------+--> one FYLO loop
                                                              --exclusive-root
```

The Go process is the only component allowed to start FYLO while the hybrid
runtime is active. The bridge:

- binds only to a loopback address;
- requires a distinct 32-byte-or-longer secret;
- authenticates each frame with HMAC-SHA-256, a timestamp, and a single-use
  nonce;
- accepts only the operations and collections frozen under
  `contracts/fylo/`;
- rejects caller-supplied roots, schema paths, backup configuration, and
  untrusted raw-file paths;
- applies frame, request, connection, attachment, and storage bounds;
- is never exposed as a public HTTP API.

The JavaScript compatibility client may start FYLO directly only outside
production and still requests `--exclusive-root`. Production startup fails
closed when the private bridge is absent.

## Released Runtime Contract

The locked FYLO release must:

- match an exact platform SHA-256 from `runtime.lock.json`;
- identify as a release build with protocol version 1, a known immutable
  commit, and the expected target;
- negotiate LF-delimited UTF-8 NDJSON with the configured request and response
  limits;
- report handshake, exclusive-root, bounded query pagination, machine-access,
  document-bucket, and embedded serverless-queue capability records;
- pin and verify the exact CHEX and TTID dependency versions reported by the
  handshake;
- pass active probes for virtual uid/gid/groups enforcement and
  document-bucket operations in addition to capability discovery.

An unknown or failed probe blocks startup.

## Supervisor Contract

The Go supervisor:

- starts one persistent FYLO child and drains stdout and stderr concurrently;
- serializes machine requests so FIFO responses cannot be mis-associated;
- bounds request and response frames and kills a child after framing loss;
- applies a deadline to every request;
- classifies a timed-out mutation as an ambiguous outcome;
- reports readiness only after a validated handshake;
- restarts a failed child within a bounded retry budget;
- revalidates identity and reacquires the exclusive-root lease after restart;
- stops accepting bridge work before bounded child shutdown.

Competing canonical or symlinked roots must fail with `EROOTLOCKED`. Normal
exit and forced termination must release the kernel-owned lease.

## Tenant and Attachment Contract

- Domain gid and mailbox uid values remain deterministic and namespace
  separated.
- Trusted group membership is derived by CADUCEUS and never accepted from a
  public request.
- Owner, same-domain member, outsider, empty-group, and mixed-domain batch
  cases are tested against the released process.
- New attachment bytes are stored in the `attachment-files` FYLO bucket.
- Bridge file ingestion accepts regular, non-symlink files only from the
  configured incoming directory.
- A single attachment and the complete attachment bucket have explicit byte
  limits.
- Integrity audits re-hash bucket bytes instead of trusting cached stamps.
- Reconciliation reports missing bucket objects and unreferenced bucket
  objects; mutation requires an explicit operator apply flag.

## Crash, Rebuild, and Recovery Contract

The qualification harness proves:

- a killed process can be replaced without overlapping root ownership;
- cancellation around a transactional batch produces no partial batch;
- an acknowledged write remains after forced restart;
- deleted indexes rebuild from document truth;
- changed attachment bytes are detected as corrupt;
- interrupted idempotent migration can be retried;
- bounded mixed read/write load does not leak processes, goroutines, or
  unbounded memory.

## Cold Backup and Restore

CADUCEUS provides an offline local-directory backup independent of external
services. A backup:

- is created while Go holds the FYLO root lease and no request is in flight;
- rejects symlinks and path escapes;
- records every path, type, size, mode, SHA-256, and supported extended
  attribute required for FYLO access and attachment integrity;
- is verified before restore;
- restores only to a new destination;
- is opened by the released FYLO binary, rebuilt, and integrity-checked before
  it can replace an active root.

FYLO S3-compatible backup is optional and is not required for CADUCEUS
operation or Phase 2 completion.

## Rollback

This slice is `backward-readable`: no new durable document shape is required.
To roll back, quiesce the legacy runtime, stop the Go owner, verify both
listeners and the bridge are closed, and restart the prior legacy digest. If
the attachment bucket or access descriptors fail verification, restore the
verified pre-cutover local backup instead.

## Completion Gate

```sh
FYLO_TEST_BINARY=/absolute/path/to/fylo bun run verify:phase2:all
```

Phase 2 is complete only when:

1. JavaScript and Go cannot become concurrent FYLO writers.
2. A cold restore reproduces tenant access decisions and attachment integrity.
3. Measured limits and unsupported topologies are published.

All three conditions are satisfied. The reproducible results, operating
limits, supported filesystems, and excluded topologies are recorded in
[`PHASE2_EVIDENCE.md`](PHASE2_EVIDENCE.md).
