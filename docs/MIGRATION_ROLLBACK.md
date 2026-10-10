# Migration and Rollback Policy

## Core Rule

Only one implementation may mutate a FYLO root. During migration, the Go
service owns FYLO and any legacy JavaScript behavior reaches it through a
private, authenticated, allowlisted bridge. Dual writers are prohibited.

## Phase 1 Runtime Foundation

Phase 1 is a code-only, non-authoritative runtime slice. The Go binary verifies
the configured FYLO artifact, schema directory, runtime lock, and synchronized
frontend bundle, but it does not start FYLO or mutate mail data. Production
container entrypoints and legacy mail behavior remain unchanged until an
explicit later cutover.

Rollback is therefore:

1. stop the Phase 1 Go process;
2. confirm both its public and operations listeners are closed;
3. restart the previously approved application digest;
4. retain the failed process logs and configuration digest for diagnosis.

No data restore is required because `caduceus migrate` is a read-only preflight
in Phase 1 and reports `mutated=false`.

## Before Every Slice

Each vertical slice must record:

- source and destination release identities;
- data and contract versions;
- a verified cold backup;
- the parity tests and failure budget;
- the point at which old code can no longer read new state;
- a bounded rollback or restore procedure.

Migration commands are idempotent, checkpointed, resumable, and mutually
exclusive with application writers. A dry run reports intended mutations and
required free space.

## Rollback Classes

| Class | Condition | Action |
| --- | --- | --- |
| Code-only | No durable format or semantic write changed | Stop new process and restart the prior digest. |
| Backward-readable | New writes remain valid for the prior release | Quiesce, verify, restart prior digest, run parity corpus. |
| Restore-required | Prior code cannot safely read new state | Quiesce, preserve failed state, restore the verified pre-migration backup, restart prior digest, verify integrity. |
| Forward-fix only | Restore would lose accepted mail and no safe inverse exists | Keep listeners bounded or read-only, ship an audited forward repair, and preserve all evidence. |

The release manifest must name the rollback class. Silence means
`restore-required`.

## Mail Safety

- Stop accepting new SMTP transactions before a destructive restore.
- Drain or snapshot the outbound queue and record every delivery lease.
- Preserve message IDs and idempotency keys to prevent duplicate delivery.
- Reconcile accepted SMTP acknowledgements against stored messages.
- Verify attachment digests and tenant ownership after restore.
- Do not delete the failed data root until the incident is closed.

## Drill Evidence

A supported release proves upgrade and rollback from every supported predecessor
using seeded multi-domain data, active queues, attachments, authentication
state, and interrupted migrations. Evidence includes timings, digests, data
counts, parity results, and operator commands.
