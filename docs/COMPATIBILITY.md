# Compatibility and Support Policy

CADUCEUS uses the support vocabulary defined in
[`BASELINE.md`](./BASELINE.md): `buildable`, `native-tested`, `preview`, and
`supported`.

## Supported Topology

The first supported topology is:

- one CADUCEUS instance;
- one local FYLO root on a filesystem with reliable locking and atomic rename;
- one persistent FYLO process with an exclusive-root lease;
- Linux OCI on `amd64` or `arm64`;
- local durable storage and operator-managed backups;
- direct SMTP delivery without a hosted delivery dependency.

Active-active writers, shared network filesystems without proven lock
semantics, Kubernetes multi-replica deployments, and dirty development
binaries are unsupported.

## Current Toolchain Lock

The 2026-08-15 lock uses Tachyon `26.33.02`, FYLO `26.33.02`, SESAME
`26.33.02`, CHEX `26.32.02`, TTID `26.32.03`, Bun `1.3.14`, and Go `1.26.5`.
Release commits, per-platform SHA-256 digests, the SESAME Go client
pseudo-version, and the JavaScript dependency-lock digest are recorded in
`runtime.lock.json`. CI, release jobs, and both container build files consume
the same exact versions.

FYLO `26.33.02` is qualified for the mail store, including machine uid/gid
authorization, document buckets, bounded pagination, and the serverless queue.
SESAME `26.33.02` pins the same FYLO release with CHEX `26.32.02` and TTID
`26.32.03`; all 14 identity black-box cases, 140 FYLO-backed database cases,
and 287 HTTP route cases pass against that released pair.
The prior SESAME/FYLO version blocker is resolved without weakening either
runtime identity check. The legacy Tachyon HTTP contract suites remain behind
the frontend/server migration boundary described below and are not part of
this runtime-pair qualification.

Tachyon `26.33.02` is installed and locked. Its exclusively client-rendered Tac
plan supports the companion-backed structural `logic` and `loop` state used by
the authenticated web client; the qualified bundle emits all 100 current
routes. Route response schemas remain enforced by CADUCEUS's independent
schema audit even though Tachyon reports the legacy `OPTIONS.schema.json`
convention as unsupported.

## Phase 1 Runtime Boundary

The Go runtime foundation is qualified with:

- a public HTTP listener on `127.0.0.1:8080` by default for the synchronized
  frontend bundle and `/api/v1/system`;
- a distinct loopback-only operations listener on `127.0.0.1:9090` by default
  for health, readiness, and metrics;
- Go 1.26.5 as the locked build toolchain;
- the exact FYLO and Tachyon artifact digests recorded in
  `runtime.lock.json`.

These defaults are development-safe and do not replace the existing production
container entrypoint. Business APIs, SMTP behavior, delivery workers, and FYLO
ownership remain on the legacy runtime until their documented migration
phases. Binding either listener beyond loopback requires explicit operator
configuration; the operations listener cannot be configured off loopback.

## Compatibility Dimensions

- API: v1 remains compatible for its documented lifetime.
- Configuration: unknown keys fail; removed keys require a deprecation cycle.
- Data: every supported prior schema has an upgrade fixture and restore proof.
- Runtime: exact binary capabilities and digests are checked before startup.
- Browser: current and previous stable Chrome, Firefox, and Safari families are
  qualified; reduced-motion and keyboard-only operation are mandatory.
- SMTP: interoperability is qualified against strict and permissive peers,
  plaintext refusal rules, STARTTLS downgrade behavior, and retry semantics.

The hosted-domain direct transport is qualified by
`scripts/verify-smtp-interoperability.mjs` against a real ESMTP socket with
EHLO, STARTTLS, Unicode MIME, dot transparency, attachments, and mixed RCPT
acceptance. `scripts/verify-mail-transport-soak.mjs` advances the production
queue state machine across 72 deterministic virtual hours with continuous
submissions, temporary 4xx deferrals, permanent 5xx failures, recipient-scoped
bounces/suppression, and duplicate-delivery assertions. CI runs both through
`bun run verify:mail-transport`. Provider-owned identities use the separately
qualified Go IMAP/SMTP connectors listed below instead of the hosted-domain
direct queue.

## Connected Mail Provider Qualification

Connected accounts are qualified at the protocol boundary with deterministic
IMAP/SMTP transcripts. A preset is supported when its advertised security and
authentication mode maps to a qualified row below; live vendor smoke tests are
release-candidate certification evidence, not a hidden runtime dependency.

| Account family | Authentication | Qualified synchronization behavior | Evidence |
| --- | --- | --- | --- |
| Gmail / Google Workspace | OAuth 2.0 XOAUTH2 | Requires `X-GM-EXT-1`; imports only All Mail, Spam, and Trash as physical sources; projects one `X-GM-MSGID` into every `X-GM-LABELS` membership; label moves and crash reconciliation retain the canonical identity; CONDSTORE and IDLE are bounded | `TestGmailSyncUsesCanonicalMessageIdentityAndLabelMemberships`, `TestGmailMoveAddsDestinationLabelRemovesInboxAndKeepsCanonicalIdentity`, `TestGmailReconcileMoveRecognizesCommittedLabelChange`, `TestGmailFailsClosedWithoutStableIdentityExtension` |
| Microsoft 365 / Outlook / shared mailboxes | OAuth 2.0 XOAUTH2 | Delegated shared-mailbox identity, read-write IMAP, authenticated SMTP send-as, QRESYNC/CONDSTORE/IDLE when advertised | `TestIMAPConnectorOpensMicrosoftSharedMailboxWithDelegatedXOAUTH2`, `TestSMTPConnectorSendsAsMicrosoftSharedMailboxWithXOAUTH2`, password-validator shared-mailbox transcript tests |
| Yahoo, iCloud, Fastmail, GMX, Yandex | App password or password over TLS/STARTTLS | Provider presets use the standards path: SPECIAL-USE folders, UIDVALIDITY, UIDPLUS, modified UTF-7, quoted or atom LIST names, NIL delimiters, and extension-negotiated QRESYNC/CONDSTORE/IDLE | provider discovery/preset tests plus the generic IMAP, mutation, SMTP, and TLS transcript suites |
| Custom IMAP/SMTP | Password over operator-selected TLS/STARTTLS | Same bounded standards path; unsafe DNS resolution, plaintext credential transport, missing Inbox, ambiguous message submission, and missing durable move/delete identities fail closed | `internal/adapters/mailprovider/password_validator_test.go`, `imap_sync_test.go`, and `smtp_submit_test.go` |

The Gmail projection deliberately follows Gmail's one-message/many-label
model: FYLO stores one raw RFC 822 bucket and one canonical provider-message
record, while mailbox queries expose that record through each current label.
Removing a label or a stale virtual folder therefore cannot delete a message
that remains in another label. Generic IMAP continues to use mailbox-scoped
UID identities because the protocol does not provide Gmail's cross-label
stable identity.

On-device message and draft translation prefers an already-installed native
browser language pack and otherwise uses checksum-pinned SMaLL-100 and OPUS
ONNX/WebAssembly fallbacks. The translation UI exposes Mailspring's complete
93-language reader catalogue. SMaLL-100 translates its 80-language subset
directly; two compact directional OPUS models load lazily and use an English
pivot only when a pair includes one of the remaining 13 languages. Real
SMaLL-100 and both OPUS directions through the authenticated production asset
route are qualified in Chrome, Firefox, and WebKit. The complete immutable
payload is 839.7 MiB, but a browser fetches only the model families required by
the selected pair and retains each loaded model in memory. Operators must
account for that bandwidth and clients without sufficient WebAssembly memory
receive an explicit capability error. The compact OPUS model has only one Mari
constituent, so Hill Mari uses that documented compatibility mapping.

## Deprecation

A supported public behavior requires:

1. an announced replacement;
2. at least one release with both paths available;
3. migration instructions and observable warnings;
4. evidence that supported clients no longer require the old path.

Security-critical behavior may be removed immediately with a security advisory
and explicit upgrade boundary.
