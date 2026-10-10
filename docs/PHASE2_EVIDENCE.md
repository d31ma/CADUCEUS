# Phase 2 FYLO Qualification Evidence

Status: complete  
Qualified: 2026-08-15

Current release qualification target: FYLO `26.33.02`, commit
`e236162508f6789f980fa7ca136a6671938030cb`. This release carries the
previously verified duration and bucket-read fixes, advertises
`serverlessQueue`, and requires CHEX `26.32.02` plus TTID `26.32.03`. The
release was re-qualified on macOS arm64 on 2026-08-15. The released binary
matched SHA-256
`2c78f91130863ad797cf7a6062aa1245fa638857fee63b8671fa9e2748974fa3`;
the complete Phase 2 gate passed with 63.0% coverage against its 62.0% floor,
including all nine bridge-backed migration, virtual uid/gid, and attachment
bucket cases. The dated sections below
remain the audit trail for the earlier release cutovers.

## Historical 26.30.06 Release Evidence

CADUCEUS previously pinned FYLO 26.30.06, protocol 1, release commit
`39f57b5cf3120c9b9b3b4ead9e749b47b76ac4f0`. The five Linux, macOS, and
Windows release digests are recorded in `runtime.lock.json`; the native
qualification used the locked `macos-arm64` artifact with SHA-256
`ae39a2b66ea9771766f3f3d6b3d0d1b01e1b3842a45aa0389535109b91bdee50`.
Startup validates the complete digest and structured release identity before
opening the bridge.

FYLO 26.30.06 advertises handshake, exclusive-root, bounded query pagination,
and whole-root backup. CADUCEUS actively probes machine uid/gid access and
document buckets because those operation families are implemented but omitted
from the current capability record. The reporting gap is tracked upstream as
[FYLO issue 88](https://github.com/d31ma/FYLO/issues/88).

Disk-pressure stress also found that macOS can first surface `getxattr` errno
316 as `EUNKNOWN` before later operations report stable `ENOSPC`, and a failed
loop can leave its response pipe open. This is tracked as
[FYLO issue 89](https://github.com/d31ma/FYLO/issues/89). CADUCEUS does not
hide the storage error: queue admission and active I/O share one deadline,
timed-out mutations remain ambiguous only after admission, forced termination
closes both child pipes, and process reaping cannot exceed the shutdown bound.

## Qualification Results

| Area | Result |
| --- | --- |
| Runtime identity | Release version, protocol, commit, target, and SHA-256 matched the lock. |
| Single writer | A second process using the canonical or symlinked root failed with `EROOTLOCKED`; lease release was proven after normal and forced exit. |
| Persistent protocol | Strict NDJSON framing, concurrent stderr draining, FIFO request association, deadlines, ambiguous mutation outcomes, bounded restart, and handshake revalidation passed. |
| Legacy bridge | A real JavaScript client used the HMAC-authenticated loopback Go owner; replay, stale timestamp, unknown operation, caller-selected root, and path-escape attempts failed closed. |
| Tenant isolation | Owner and same-domain group access passed; outsiders, empty groups, and mixed-domain batches were denied by released FYLO uid/gid enforcement. |
| Attachments | `attachment-files` ingestion, regular-file and path restrictions, per-object and bucket quota checks, digest corruption detection, orphan reporting, and explicit repair passed. |
| Recovery | Acknowledged writes survived forced restart; deleted indexes rebuilt from document truth; a 2,048-document canceled batch had no partial commit. |
| Cold restore | Manifest, SHA-256, modes, and supported xattrs verified; restore to a new root preserved tenant access decisions and attachment integrity and reopened under the released binary. |
| Migration retry | An interrupted idempotent domain migration retried to exactly one completed migration. |
| Mixed load | Eight workers each completed 20 put/get cycles without leaked FYLO processes or goroutines; the complete scenario passed 10/10 consecutive normal-capacity stress runs. |
| Container | The production image built, returned HTTP 200 from health, ran exactly one `fylo exec --loop --exclusive-root`, and stopped the bridge and Tachyon cleanly. |

The Phase 2 critical-package black-box profile is 66.0% against a 65% floor.
The Go foundation profile is 86.9% against an 80% floor, with zero functions
above the CRAP score ceiling of 15. `go test -race ./...`, the released-runtime
Go suite, the bridge-backed JavaScript tests, strict TypeScript validation, and
the complete Phase 0/Phase 1 compatibility and browser gates pass.

## Enforced Limits

| Boundary | Default |
| --- | ---: |
| Machine request | 1 MiB |
| Machine response | 8 MiB |
| Request deadline | 10 seconds |
| Automatic restart budget | 3 |
| Bridge clock skew and nonce lifetime | 30 seconds |
| Bridge secret | At least 32 characters |
| Individual attachment | 50 MiB |
| Attachment bucket | 10 GiB |

These are deployment defaults and fail validation outside their permitted
ranges. They are not claims about maximum FYLO throughput.

## Supported and Unsupported Topologies

Phase 2 supports one CADUCEUS instance, one persistent FYLO child, and one
local POSIX filesystem. Linux filesystem ownership and macOS/Linux extended
attributes are preserved where the host supports them. The service account
must be able to assign the configured virtual GIDs; the supplied container adds
only `CAP_CHOWN`.

The following remain unsupported:

- active-active instances or two authoritative writers for one root;
- shared or network filesystems, including NFS and SMB;
- a FYLO root whose parent is not writable by the service account;
- a public, proxied, non-loopback, or cross-host persistence bridge;
- live restore into an active root or restore over an existing destination;
- Phase 2 local backup/restore on Windows;
- treating S3-compatible storage or any external backup service as required.

Operators must stop the service before backup, restore, or applied
reconciliation so the command can acquire the same exclusive-root lease.

## Reproduce

```sh
FYLO_TEST_BINARY=/absolute/path/to/fylo bun run verify:phase2:all
docker build -t caduceus:phase2 .
```

The CI `phase2` job installs the same locked release from its upstream checksum
manifest and runs the non-browser qualification on Linux. Phase 0 browser
interaction and responsive-matrix jobs remain independent required gates.

## FYLO 26.31.06 (Rust engine) — 2026-08-03

Re-qualified against the Rust rewrite, published artifact
`0eaa740eaf536c1fe5fe8d9eb24a42469b40566593a3641ffa693fc68b111a66`
(macos-arm64), commit `7724c3915d12ae4fa7cd5cea4f9830adf5bbb6b3`.

**Passing:** all six Phase 2 blackbox tests, every `internal/...` Go test, and
311 storage-level JS tests. A root written by 26.30.06 is read correctly by
26.31.06 — documents, queries, and bucket metadata all resolve — so the upgrade
is forward-compatible for existing data.

**Lost:** whole-root backup. The cutover removed the `wholeRootBackup`
capability, the `fylo backup ...` CLI commands, and the `backupStatus` and
`backupReconcile` machine operations, so `caduceus backup create|verify|restore`
has no engine support. Reported as
[FYLO #98](https://github.com/d31ma/FYLO/issues/98). The runtime lock records
the loss under `artifacts.fylo.missingCapabilities` and the Phase 2 gate refuses
to pass unless that record exists. Coverage floor moved 65 → 62 for the same
reason, with the code still present and compiled.

**Blocked:** everything behind SESAME. The Rust engine emits a fractional
`durationMs` and SESAME's client types it as `int64`, so the identity engine
cannot complete its first handshake and never starts. That is
[FYLO #97](https://github.com/d31ma/FYLO/issues/97), already open; CADUCEUS
evidence added there. Phase 3 tests and the API/route suites cannot run on
26.31.06 until it is fixed.

**Two CADUCEUS defects this upgrade exposed**, both since fixed: the Phase 2
qualification and the vendored node client each derived a bucket object's path
from FYLO's private shard layout, which changed from the identifier's first two
characters to its last two. Both now locate the object instead. The underlying
reason a consumer had to touch that layout at all — no protocol operation
returns bucket bytes — is [FYLO #99](https://github.com/d31ma/FYLO/issues/99).

### Verification of the upstream fixes — 2026-08-03

Built the local FYLO fix branch (`7724c391` plus its working-tree changes) with
release identity and re-ran the CADUCEUS gates against it.

| Report | Outcome |
| --- | --- |
| [#97](https://github.com/d31ma/FYLO/issues/97) `durationMs` | **Fixed.** Whole number in every envelope. SESAME now decodes the handshake and gets past it. |
| [#98](https://github.com/d31ma/FYLO/issues/98) backup surface | **Resolved as documentation, not restoration.** ADR 0007 retires filesystem-external backup deliberately; the four operations and the CLI now name the ADR and point at `docs/operations/filesystem-snapshot-restore.md`. The `wholeRootBackup` capability stays absent by design. `caduceus backup` still has no engine support and the replacement is a filesystem procedure. |
| [#99](https://github.com/d31ma/FYLO/issues/99) bucket reads | **Fixed.** `getFileData` returns content inline as base64, or writes to a caller-supplied `path` when the object exceeds one response frame; advertised as `documentBuckets.getOutputs`. Verified against a 12 MB object. The fractional `lastModified` was fixed in a follow-up, along with `updatedAt`, `mtime`, and `deletedAt`; no non-integral number remains in any payload or envelope. A root written by the fractional build reads back with an integer and stays queryable. |
| [#96](https://github.com/d31ma/FYLO/issues/96) latency | **Fixed, and then some.** 200 sequential `putData`: p50 188.8 ms on the published 26.31.06, 4.0 ms on the fix build, against 9.0 ms on the 26.30.06 JavaScript runtime. Write cost is now flat with collection size (first 50 avg 4.5 ms, last 50 avg 4.3 ms) instead of growing. |

Gate results on the fix build: Phase 2 qualification 6/6 in **5.4 s** (34 s on
the published build), every `internal/...` Go test passing, and 311 storage
JS tests in **6.6 s** (190 s on the published build).

At that point, anything behind SESAME was blocked: `26.30.07` pinned FYLO to exactly
`26.30.06`, so it now fails on its own version check rather than on the decode
error. That is a SESAME release concern, not a FYLO defect.

The fixes described above are published in `26.33.01`; the runtime lock now
supersedes it with `26.33.02` and its five platform digests. SESAME `26.33.02`
pins the same FYLO release, and the Phase 3 gate now passes against the pair.
