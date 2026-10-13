# RFC 0001: Optional External Account Connectivity

- Status: Accepted for incremental delivery
- Date: 2026-08-15
- Owners: Accounts, Mailbox, Delivery, Contacts, Calendar

## Summary

CADUCEUS remains a complete standalone multi-domain mail server. An authenticated
mailbox owner may additionally connect external mailboxes and see them beside
hosted mailboxes in the same client. The Go runtime owns provider credentials,
network validation, synchronization, retry state, and all FYLO writes. The
browser and the legacy Tachyon adapter receive redacted account projections
only.

This closes the Mailspring account-sidebar, onboarding, unified-account,
provider-sync, shared-mailbox, CardDAV, and external-calendar gaps without
making Gmail, Microsoft, or any other provider a deployment dependency.

## User scenarios

1. A user adds a Gmail, Microsoft 365, Outlook.com, or standards-based
   IMAP/SMTP account and sees validation progress before it is saved.
2. A user may rely on discovery or enter explicit IMAP and SMTP endpoints,
   ports, usernames, and TLS modes. Plaintext transport is rejected except for
   an explicitly enabled loopback development endpoint.
3. A Microsoft user may connect a shared mailbox by authenticating as the
   delegate and naming the shared address. CADUCEUS separately reports read and
   send-as capability.
4. Each external account has a label, color, order, aliases, default sender,
   provider folders, unread policy, and visible sync state.
5. Unified views aggregate hosted and external accounts. Selecting one account
   scopes mailbox, search, contacts, calendar, notifications, and compose.
6. Authentication failures request reconnection without deleting local mail.
   Transient failures retry durably with bounded exponential backoff. A user
   can pause, resume, retry, inspect redacted diagnostics, or remove an account.
7. Removing an account revokes its credentials immediately, cancels queued
   work, and asynchronously purges its imported data. It never affects another
   account owned by the same user or another user in the same domain.

## Providers and discovery

The public provider catalog is fixed in v1:

- `gmail`: OAuth 2.0; Gmail labels; Google People and Google Calendar when the
  operator configures the corresponding client credentials.
- `microsoft`: OAuth 2.0; Microsoft 365, Outlook.com, and delegated shared
  mailboxes; Microsoft Graph contacts and calendars when configured.
- `imap`: RFC IMAP plus SMTP submission. Presets cover common services, while
  SRV, MX-derived presets, and well-known configuration may fill suggestions.
  The user must confirm the final endpoints before credentials are tested.

Discovery is advisory and never sends a password or refresh token. Validation
performs DNS and TCP/TLS work from the Go runtime with strict address, redirect,
certificate, response-size, and time bounds. It blocks link-local, multicast,
unspecified, and private destinations unless an operator explicitly enables a
private network CIDR. DNS rebinding is checked again at connection time.

## Domain model

An `ExternalAccount` is owned by one authenticated SESAME tenant/principal, not
by an email address or mail domain. Its stable identity is
`(tenantId, principalId, id)`; the owner's CADUCEUS mailbox is display metadata.
The same remote address may
be connected by different CADUCEUS users, but one owner cannot create a second
active account with the same provider and remote mailbox.

Public account fields are defined by
`contracts/http/external-accounts.v1.schema.json`. Secret material is stored in
a separate encrypted record and is never present in list, detail, event, log,
or error responses. FYLO access metadata uses a deterministic principal uid and
opaque provider-account gid; application-level owner checks remain mandatory.
A provider domain such as `gmail.com` is never an authorization group.

Account lifecycle:

```text
draft -> validating -> ready -> syncing -> idle
              |                    |        |
              +-> auth_required <--+        +-> degraded
              +-> failed                    +-> paused

any persisted state -> deleting -> deleted
```

- `draft` is browser-local and never stores credentials.
- `validating` is a short-lived server transaction.
- `ready`, `syncing`, `idle`, `auth_required`, `degraded`, and `paused` are
  durable.
- `deleting` is durable and immediately scheduled. The worker acquires the
  account lease, purges imported folders, message indexes, raw RFC 822 buckets,
  mutations, and sync state, then removes the schedule and public account
  record last. A crash leaves the tombstone discoverable and retryable.
  `deleted` is an audit event, not a returned account.
- State transitions use an expected revision. Concurrent or stale mutations
  return `409 account_revision_conflict`.

Sync progress is monotonic within one run and contains phase, completed units,
total units when known, and timestamps. Stable phases are `connecting`,
`folders`, `headers`, `bodies`, `contacts`, `calendar`, and `complete`.

## Go module boundaries

New business code lives under `internal/accounts/`:

```go
type Owner struct {
    TenantID string
    PrincipalID string
    Address string // display metadata only
}

type Repository interface {
    Create(context.Context, ExternalAccount, EncryptedSecret) (ExternalAccount, error)
    Get(context.Context, Owner, AccountID) (ExternalAccount, error)
    List(context.Context, Owner) ([]ExternalAccount, error)
    Update(context.Context, ExternalAccount, uint64) (ExternalAccount, error)
    MarkDeleting(context.Context, Owner, AccountID, uint64) error
}

type Connector interface {
    Discover(context.Context, DiscoveryRequest) (DiscoveryResult, error)
    Validate(context.Context, CredentialMaterial, EndpointPolicy) (Capabilities, error)
    Sync(context.Context, SyncRequest, ChangeSink) error
}

type SecretVault interface {
    Seal(context.Context, Owner, AccountID, Provider, CredentialMaterial) (EncryptedSecret, error)
    Open(context.Context, Owner, AccountID, Provider, EncryptedSecret) (CredentialMaterial, error)
    Destroy(context.Context, Owner, AccountID, Provider) error
}
```

Feature services depend on these consumer-owned ports. FYLO, OAuth, IMAP,
SMTP, CardDAV, and calendar implementations live under `internal/adapters/`.
Worker scheduling, network policy, metrics, and encryption key loading live
under `internal/platform/`. The legacy `server/` route may only forward a
versioned request to the Go boundary during migration.

## HTTP contract

All routes are under `/api/v1` and require a live SESAME bearer session:

- `GET /provider-catalog`
- `POST /provider-discovery`
- `POST /external-accounts/validate`
- `GET, POST /external-accounts`
- `POST /external-accounts/oauth/microsoft/begin`
- `POST /external-accounts/oauth/microsoft/complete`
- `POST /external-accounts/oauth/google/begin`
- `POST /external-accounts/oauth/google/complete`
- `GET, PATCH, DELETE /external-accounts/{id}`
- `POST /external-accounts/{id}/retry`
- `POST /external-accounts/{id}/pause`
- `POST /external-accounts/{id}/resume`
- `GET /external-accounts/{id}/diagnostics`
- `GET /external-accounts/events` as a bounded SSE stream

Microsoft OAuth initiation stores a single-use, ten-minute, owner-bound PKCE
transaction in FYLO. The callback uses only the exact registered redirect URI,
never exposes provider tokens to browser JavaScript, and validates delegated
IMAP Full Access plus SMTP authentication before the account becomes ready.
The user explicitly enters a shared mailbox address; Microsoft does not expose
a complete shared-mailbox discovery list through the delegated IMAP contract.

## Synchronization invariants

- One durable worker lease exists per account. A restart may repeat a remote
  read, but importing a provider object is idempotent by `(accountId,
  providerStableId, providerRevision)`.
- Provider cursors are committed only after all corresponding FYLO mutations
  commit. An ambiguous mutation is reconciled before the cursor advances.
- Local operations are journaled before remote mutation. Retry distinguishes
  authentication, permanent policy, throttling, and transient transport errors.
- Connector concurrency, bytes, pages, folder depth, recipients, redirects,
  and wall time are bounded. Backoff honors provider retry hints and adds
  jitter.
- Imported messages retain raw RFC 822 source and attachments in the existing
  FYLO buckets. Provider objects never become a second source of truth for
  hosted mail.
- No two authoritative FYLO writers use `/data/caduceus`.

## Security and privacy

- Credentials are encrypted with an operator-supplied 256-bit key or a
  deployment key stored mode `0600`; ciphertext is bound to owner, account,
  provider, and key version as authenticated data.
- OAuth refresh tokens and passwords are redacted before logs and errors.
- Discovery and connectors enforce SSRF controls after every DNS resolution
  and redirect. TLS verification is mandatory by default.
- Every API lookup includes the authenticated owner. Unknown and foreign IDs
  return the same `404` response.
- Diagnostics expose stable error codes, timestamps, retry time, and redacted
  protocol stages, never remote message content or credentials.
- Operator policy may disable all external connectivity or individual
  providers. With connectivity disabled, CADUCEUS behavior is unchanged.

## Observable acceptance criteria

1. A real IMAP/SMTP fixture can be discovered, validated, persisted, synced,
   paused, resumed, reauthenticated, and removed through public routes.
2. Microsoft and Google OAuth fixtures prove PKCE, state expiry, refresh,
   reconnect, and redaction. Protocol fixtures separately prove delegated
   shared-mailbox read/send-as identity and Gmail label-capable XOAUTH2.
3. Two users in the same hosted domain cannot list, read, mutate, diagnose, or
   receive events for each other's accounts, secrets, cursors, or imported data.
4. A killed worker resumes from its durable cursor without duplicates or lost
   acknowledged actions.
5. The account sidebar, settings, onboarding, composer, search, contacts, and
   calendar work at phone, tablet, desktop, keyboard-only, screen-reader, dark,
   high-contrast, reduced-motion, and 400% reflow profiles.
6. `bun run audit:mailspring-parity:complete` cites these tests before any
   affected ledger item becomes `implemented`.

## Delivery slices

1. Redacted account registry, manual IMAP/SMTP validation, durable sync state,
   settings UI, and account-scoped folder/message APIs. Complete.
2. IMAP mailbox and SMTP submission synchronization with category mapping.
   IMAP UID import, bounded flag/expunge/folder reconciliation, UIDVALIDITY
   resets, UIDVALIDITY-bound mutations, UID-scoped delete expunge with safe
   rollback when UIDPLUS is unavailable, crash-safe and revision-independent
   idempotent mutation replay, FYLO
   projection, unified inbox/search/RFC-thread reads, exact account-scoped
   desktop and responsive folder navigation, compare-and-swap sidebar
   color/order controls, delimiter-aware collapsible folder hierarchy,
   durable create-child/rename/delete folder actions, modified UTF-7 mailbox
   names, exact per-folder EML/mbox export, account sync/pause/settings actions,
   composer sender selection, and
   journal-first authenticated SMTP submission are complete.
   SMTP delivery uses the account lease, encrypted server-owned credentials,
   bounded raw-message buckets, idempotency keys, and an explicit ambiguous
   terminal state after a post-DATA connection loss. Provider quick and bulk
   read/star/move/delete actions use the durable owner action journal, while
   read, star, and move operations support complete mixed-provider undo and redo
   with exact mutation status and conflict recovery. MOVE requires UIDPLUS
   `COPYUID`; CADUCEUS persists the assigned destination UID before projection,
   reconciles it after crashes, and uses that identity for connected-account
   Send & Archive plus Undo Send restoration. Incremental unread Inbox imports enter a
   scheduler-owned notification outbox before checkpoint advance; initial
   hydration and UIDVALIDITY resets are suppressed, and a signed bounded worker
   applies the same privacy, action, sound, and exact-account policy as hosted
   Web Push. The unified SSE stream maintains independent hosted/provider
   cursors and replays the oldest unseen bounded slice. Connected schedules can
   be edited in place under a monotonic provider-submission revision. FYLO
   stages one bounded MIME bucket per envelope recipient, retains the previous
   authoritative batch until an atomic commit, and automatically aborts an
   abandoned edit after its bounded lease. The same clean-Sent/provider-delivery
   split supports self-hosted encrypted open/link tracking for one or many
   recipients. Capability-negotiated QRESYNC now applies bounded `VANISHED`
   and flag deltas with an atomically advanced durable `HIGHESTMODSEQ`.
   CONDSTORE-only servers use restart-safe, bounded UID range sweeps and retain
   explicit presence windows for expunge detection; servers without either
   extension keep the legacy bounded reconciliation path. A bounded,
   independently supervised IDLE watcher now converts Inbox notifications into
   durable schedule wakes without becoming a second FYLO writer; unsupported
   servers retain the polling fallback.
3. Gmail OAuth, native Google Calendar API v3 sync/write, and native Google
   People contact synchronization/write are complete. Gmail mail sync now
   requires `X-GM-EXT-1`, imports canonical physical sources, stores one FYLO
   message per `X-GM-MSGID`, projects every `X-GM-LABELS` membership, and
   reconciles label moves without changing the canonical identity.
4. Microsoft OAuth, shared mailboxes, and native Microsoft Graph calendar
   sync/write support are complete. Native Microsoft Graph contact-folder
   synchronization and writes are complete. Password accounts can use the same
   writable CalDAV and CardDAV paths when their provider exposes them.
5. Offline conflict UX, diagnostics, operational runbooks, and complete parity
   evidence.

## Out of scope

- Requiring any external provider for hosted CADUCEUS mail.
- POP3, Exchange ActiveSync, or importing Mailspring's GPL sync engine.
- Browser-side provider credentials or direct browser-to-provider connections.
- Silent fallback from verified TLS to plaintext or insecure TLS. Manual
  plaintext or certificate bypass remains an explicit, policy-controlled
  compatibility setting and is never selected by discovery.
