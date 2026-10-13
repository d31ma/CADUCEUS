# API v1 Contract

The machine-readable route inventory is
[`contracts/http/routes.v1.json`](../contracts/http/routes.v1.json). Stable
errors are defined in
[`contracts/errors/v1.json`](../contracts/errors/v1.json).

## Versioning

- Canonical application routes use `/api/v1`.
- The current unversioned paths are compatibility aliases through migration
  Phase 7.
- Internal HMAC routes are versioned with the same prefix but are never public
  extension points.
- Breaking request or response changes require `/api/v2`; adding optional
  fields is compatible.
- Clients must ignore unknown response fields and branch on stable error
  `code`, never on human-readable `detail`.

## Authentication

- User endpoints use a short-lived bearer session.
- Primary authentication endpoints may be public but are rate-limited.
- Continuation endpoints accept a bounded, single-purpose authentication
  transaction.
- Inbound delivery and queue ticks use HMAC-SHA256 over the exact body.
- Resource authorization always derives tenant scope from verified claims;
  request bodies cannot expand it.

## Protocol Rules

### Mailbox rules

`GET /rules` lists only the authenticated primary mailbox owner's rules in
processing order. `POST /rules` accepts an exact owned primary or alias
`mailbox`; `PUT` and `DELETE /rules/{id}` hide foreign identifiers with 404.
`PUT /rules/order` requires every owned rule ID exactly once. `POST
/rules/preview` accepts either an owned `ruleId` or a validated draft plus a
sample message and returns condition results and planned actions without
executing them.

- JSON requests use UTF-8 and reject duplicate or unknown security-sensitive
  fields.
- Request size, recipient count, attachment count, and attachment bytes are
  bounded before persistence.
- Timestamps use RFC 3339 UTC.
- IDs are opaque and case-sensitive.
- Collection responses define deterministic ordering and bounded pagination.
- `429` and retryable `503` responses include a retry hint.
- Every response carries a request identifier; logs and audit records use the
  same identifier without recording secrets.

## Runtime Endpoints

- `GET /api/v1/system` returns
  [`contracts/http/system.v1.schema.json`](../contracts/http/system.v1.schema.json).
- `GET /system` remains its unversioned compatibility alias.
- `GET /healthz` is process liveness; `/health` remains its compatibility alias.
- `GET /readyz` is false before startup completes and from the beginning of
  graceful shutdown.
- `GET /metrics` exists only on the loopback operations listener, which defaults
  to `127.0.0.1:9090`.

## User Preferences

- `GET /users/me/preferences` returns normalized `unreadBadgeScope` and
  `unreadBadgeDomain` fields alongside the user's other preferences. It also
  returns `newMailNotificationDomains`,
  `newMailNotificationExternalAccounts`, `newMailNotificationPrivacy`, and
  `newMailNotificationActions`.
- `PUT /users/me/preferences` accepts a partial update. `unreadBadgeScope` is
  either `all` or `domain`; a focused `unreadBadgeDomain` must be one of the
  authenticated user's assigned domains. The same persisted scope controls
  foreground Badging API updates, the tab-title fallback, and authoritative
  unread counts in Web Push payloads.
- `newMailNotificationDomains` is an exact subset of the authenticated user's
  assigned domains. `newMailNotificationExternalAccounts` is an exact subset
  of the same owner's connected-account IDs; the Go boundary reauthorizes that
  ownership before a change is stored. An empty list disables alerts for that
  account class.
  `newMailNotificationPrivacy` is `full`, `sender`, or `private`, and
  `newMailNotificationActions` controls the allow-listed Mark read and Archive
  buttons. The existing `notificationSounds.new-mail` value controls whether
  background platform notifications are silent.
- Provider sync inserts unread incremental Inbox arrivals into a scheduler-only
  FYLO outbox before advancing its checkpoint. Initial hydration and
  UIDVALIDITY reset batches do not alert. The signed internal
  `POST /notifications/provider/tick` leases a bounded batch, applies the same
  privacy/action/sound policy as hosted mail, retries delivery failures with
  capped backoff, and prunes terminal markers after 30 days.
- The IMAP worker negotiates capabilities after authentication. QRESYNC applies
  bounded flag and `VANISHED` deltas in the same FYLO commit that advances the
  durable `HIGHESTMODSEQ`. CONDSTORE-only servers checkpoint bounded UID-range
  sweeps against a fixed target and retain explicit presence windows for
  expunge detection. Servers without either extension keep the bounded legacy
  reconciliation path; every path remains fenced by UIDVALIDITY.
- An independent bounded watcher negotiates IMAP IDLE, selects the provider's
  Inbox, and converts mailbox notifications into `NextRunAt=now` scheduler
  wakes. It never commits message data or holds the synchronization lease;
  unsupported and interrupted watches retain the ordinary polling fallback.

## Trusted Extensions

- `POST /extensions/inspect` validates a local plugin or declarative theme and
  returns the normalized manifest plus an immutable SHA-256 digest without
  persisting it.
- `POST /extensions` installs the bundle disabled only when the caller echoes
  that exact digest. `GET /extensions` returns redacted exact-owner records.
- `GET /extensions/{id}` is the owner-scoped runtime read. `PUT` accepts only
  the enabled state; `DELETE` removes the exact-owner package.
- Plugin permissions and contribution slots are closed allowlists. Themes
  contain no executable source or arbitrary CSS.

## Durable Drafts

- `POST /drafts` creates a draft idempotently from a stable `clientId`.
- `GET /drafts/{id}` returns the owning mailbox's latest version, including the
  attachment bytes required to recover an interrupted compose session.
- `PUT /drafts/{id}` performs a compare-and-swap using the last acknowledged
  integer `version`; stale writers receive `409` and cannot overwrite a newer
  browser session.
- `DELETE /drafts/{id}` permanently discards the draft and its FYLO bucket
  objects. Draft access requires exact mailbox ownership within tenant scope.

## Reconnect-safe Mail Actions

- `POST /inbox/actions` accepts an optional UUID `clientRequestId` alongside
  `ids` and `patch`. Repeating the same owner, UUID, message set, and patch
  returns the existing journal result without applying the mutation twice.
- Reusing a UUID for different action content returns `409`. The browser keeps
  queued action metadata without credentials, replays it in order after an
  online or Background Sync signal, and clears it on sign-out.
- Patches may change `folder`, `read`, or `starred`, or atomically add/remove
  bounded traversal-safe `labels`. Label additions and removals cannot overlap;
  undo and redo journal each message's exact prior label set.
- Hosted and connected-account IDs may participate in one action. Provider
  before/after states and mutation IDs remain in the owner-scoped journal;
  deterministic per-transition generations make retries idempotent without
  collapsing a later redo into the original forward command. Direct provider
  label add/remove patches remain rejected because generic IMAP folders and
  labels are not one portable mutation contract. Gmail folder moves use native
  `X-GM-LABELS` mutation and preserve the message's canonical `X-GM-MSGID`
  identity across every current label.

## Mailbox Categories

- `GET /categories` returns exact-owner account capabilities plus folders and
  labels with stable path, kind, display name, parent path, depth, and optional
  system role.
- `POST /categories` idempotently creates one nested folder or label in an
  assigned domain. Paths are normalized and traversal-safe; a path cannot be
  both kinds and each mailbox catalog is bounded.

## Delivery Insights

- `POST /send` may select an exact active send-capable connected-account
  mailbox in `from`. The Go owner journals bounded RFC 822 source in FYLO and
  submits with server-owned credentials; queued, sent, failed, cancelled, and
  ambiguous states never expose the source or credentials.
- `POST /send` accepts strict optional `openTracking` and `linkTracking`
  booleans. Both default off and require HTML when enabled. Tracking identities
  are generated only by the server.
- Stored Sent HTML and retained MIME source stay free of pixels and redirect
  URLs. Immediately before SMTP delivery, the adapter creates a distinct
  encrypted capability and HTML transformation for each normalized recipient.
- `GET /track/open?t=...` records an open and returns a fixed transparent GIF.
  `GET /track/click?t=...` redirects only when the capability's exact message,
  recipient, link index, and destination match the stored outbound link.
- `GET /activity` requires bearer authentication and derives mailbox and alias
  ownership from the session. It exposes bounded aggregates, daily and subject
  analytics, unseen event counts, and a newest-first event feed with
  `all`/`open`/`click` and time-range filters.
- Open and click events are delivery signals, not proof that a human read the
  message: mail-provider proxies, security scanners, and link prefetchers can
  produce events. Clients must disclose this before the user opts in.

## Follow-up Reminders

- `POST /send` may include a future `remindAt`; scheduled delivery requires the
  reminder to occur after `sendAt`. `POST /reminders` adds or updates a reminder
  for an existing exact-owner Sent message.
- A matching inbound participant reply completes the reminder by canonical RFC
  thread identity. Subject normalization is used only for historical messages
  whose RFC lineage was not retained.
- `GET /reminders` lists the mailbox owner's scheduled and due reminders;
  `DELETE /reminders` dismisses one without exposing foreign record existence.
- HMAC-authenticated `POST /reminders/tick` is internal. Its supervised,
  non-overlapping caller scans every mailbox and reports bounded due, surfaced,
  reconciled, and failed counts through secret-free status logs.

## Contacts and vCard

- `GET /contacts` lists the authenticated mailbox's contacts and accepts
  optional `q`, `source`, and `groupId` filters. Sources are `manual`, `mail`,
  `vcard`, and `carddav`.
- `POST /contacts` creates a personal contact; `GET`, `PUT`, and `DELETE`
  `/contacts/{id}` require exact mailbox ownership.
- `GET /contacts/address-books` lists only active CardDAV, Google People, and
  Microsoft Graph books owned by the authenticated mailbox. The projection
  contains display metadata, write capability, and opaque account/book IDs;
  provider URLs and synchronization cursors never cross the bridge.
- `POST /contacts` with provider-synchronized source `"carddav"` (the stable
  compatibility token), an exact owned `accountId`, an
  active `addressBookId`, and an `idempotencyKey` queues remote creation. The
  server generates an opaque remote placeholder and stable contact identity;
  callers cannot choose provider paths. The connector maps it to the
  provider-native identity after synchronization.
- `PUT` and `DELETE /contacts/{id}` queue conditional provider mutations when
  the contact source is the provider-synchronized `carddav` compatibility
  token. `GET
  /contacts/{id}/mutations/{mutation_id}` returns only that contact's
  owner-scoped durable mutation state.
- `GET|POST /contacts/groups` and `PUT|DELETE /contacts/groups/{id}` manage
  mailbox-owned groups. Deleting a group retains its contacts and removes only
  the membership.
- `GET /contacts/suggestions?q=...` returns bounded email-address completion
  candidates ordered by interaction recency and frequency.
- `POST /contacts/import` accepts vCard text plus optional owned group IDs.
  `POST /contacts/export` returns Base64-encoded vCard content for all contacts
  or the supplied owned contact IDs.
- An optional HTTPS CardDAV URL on an external IMAP account reuses that
  account's encrypted password or app password. The Go worker performs
  authenticated principal/home/address-book discovery and bounded vCard
  synchronization under the account lease. FYLO commits a complete refresh
  through an atomic generation switch scoped by SESAME tenant, principal, and
  account identity. Subsequent refreshes use RFC 6578 sync tokens and multiget
  only changed vCards, with a safe full-refresh fallback for expired tokens.
  Synced contacts appear in ordinary contact search and recipient completion.
  Conditional remote writes and their durable FYLO conflict journal are
  exposed through the authenticated contact editing boundary. Creation uses
  `If-None-Match: *`; updates and deletes use the last observed ETag. A retry
  after an ambiguous response fetches and compares the remote vCard before it
  can be considered applied. The UI polls owner-scoped mutation status and,
  on a stale-ETag conflict, lets the user either review the refreshed server
  version before editing or keep it unchanged. Parser and merge fixtures cover
  RFC vCard 3/4 forms, URI-valued fields, and grouped Apple-style properties.
- Gmail and Google Workspace accounts synchronize Google People connections
  with persisted sync tokens and a full-refresh fallback when Google expires a
  token. Microsoft accounts discover the default and named Graph contact
  folders, using delta links where Graph supports them and bounded full refresh
  otherwise. Both adapters project structured contacts through the same
  exact-owner vCard/FYLO boundary, expose only provider-declared writable books,
  use native source ETags for conditional changes, and journal an opaque
  provider marker so a retried create is reconciled instead of duplicated.

## External calendars

- An optional HTTPS CalDAV URL and username on an external IMAP account reuse
  that account's encrypted password or app password. Validation discovers the
  authenticated principal, calendar home, and at least one calendar without
  sending mail or persisting credentials in browser state.
- The Go-owned account worker performs bounded initial VEVENT queries and RFC
  6578 incremental synchronization under the account lease. Expired sync
  tokens fall back to a complete refresh; unsafe endpoints, redirects,
  response sizes, collection counts, object counts, and iCalendar bytes fail
  closed.
- Gmail and Google Workspace OAuth accounts use Google Calendar API v3 with a
  persisted per-calendar sync token and a safe full-refresh fallback when
  Google expires that token. Microsoft OAuth accounts use the Microsoft Graph
  v1 calendar surface, including delegated shared-mailbox identity, bounded
  pagination, and complete per-calendar refreshes. Both adapters project timed
  and all-day events, recurrence masters and exceptions, cancellations,
  attendees, reminders, collection colors, and remote write permissions into
  the same exact-owner RFC 5545 boundary as CalDAV.
- FYLO publishes calendar collections and resources through one exact-owner
  generation switch. The internal authenticated bridge returns raw iCalendar
  only to the trusted legacy server adapter; the browser receives normalized
  event fields, opaque account/calendar identities, and display colors.
- `GET /calendar/calendars` returns the writable provider collections owned by
  the authenticated mailbox without exposing remote URLs, credentials, ETags,
  sync tokens, or raw iCalendar.
- `GET /calendar/events` merges provider events with local events, suppresses
  an imported invitation copy with the same iCalendar UID, expands recurring
  events inside the requested range, and returns stable opaque provider event
  identities. `POST /calendar/events` accepts a provider `calendarId` to create
  a remote event; `PUT` and `DELETE /calendar/events/{id}` update or delete an
  exact-owned provider event or recurrence instance. These operations require
  idempotency keys and enter an exact-owner durable mutation journal before the
  worker performs provider-appropriate conditional writes. CalDAV and Google
  use deterministic create identities and reconcile ambiguous retries;
  Microsoft Graph receives a deterministic transaction ID. Stale ETags return
  a conflict rather than overwriting a remote edit.

## Mailbox Interchange

- `GET /inbox/{id}` and the mutable-field `PUT` response may include the
  derived `githubUrl` capability for an exact GitHub notification. The server
  accepts only credential-free HTTPS targets on `github.com` or
  `www.github.com`; the field is not persisted from untrusted message input.
- `GET /inbox/{id}/source` returns the authorized message's exact retained RFC
  822 source for inspection and `.eml` download.
- `GET /inbox/{id}/attachments/{attachment_id}` returns one attachment's
  normalized metadata and base64 bytes after exact mailbox and parent-message
  authorization. No public attachment URL is part of the v1 contract.
- `POST /inbox/{id}/images` accepts one to six remote image URLs that must
  occur in that exact owned message. It returns only bounded PNG, JPEG, GIF,
  WebP, or AVIF bytes after public-address DNS validation, address pinning,
  redirect revalidation, standard-port enforcement, MIME-signature checks,
  and per-user rate limiting. The browser renders the result as inert data;
  it never connects to the remote image host or forwards cookies, a referrer,
  its address, or its user agent.
- `POST /inbox/import/eml` accepts one base64-encoded RFC 822 message (up to
  50 MiB) plus an explicit authorized `mailbox` and destination `folder`.
  CADUCEUS retains the exact source bytes and derives searchable text, safe
  HTML, attachments, sender-security metadata, unsubscribe metadata, and RFC
  thread identity. Each file commits atomically; import never executes SMTP
  routes, forwarding, webhooks, inbox rules, or new-mail notifications.
- `POST /inbox/actions` accepts one through 100 unique owned message IDs and a
  patch containing `folder`, `read`, `starred`, or bounded label add/remove
  operations. It authorizes the
  complete set before writing, journals the exact affected fields in FYLO, and
  returns an opaque action ID with a ten-minute expiry. `POST
  /inbox/actions/{id}/undo` and `/redo` are exact-owner, idempotent transitions.
  They complete an interrupted transition on retry, retain unrelated later
  edits, and fail with `409` when the same field has changed or a provider
  rejected the journaled command. The client offers an explicit authoritative
  refresh after that conflict.
- `GET /inbox/export/rfc822?folder=...&domain=...` and the mutually exclusive
  `?folder=...&account=...` form are the bounded binary source
  feed used for folder export. It begins with `CADUCEUS-RFC822-FRAMES/1`, then
  carries repeated big-endian metadata/source lengths, UTF-8 metadata, and the
  exact source bytes. The route uses immutable FYLO query pages and keeps only
  one message source resident at a time. The connected-account form resolves
  one exact owner-scoped provider folder and pages its indexed messages before
  reading each raw source.
- The authenticated client converts that feed either to the mboxrd dialect or
  to a store-only ZIP64 archive with one collision-safe `.eml` entry per
  message. Mbox output uses LF line endings, UTC asctime separators,
  Return-Path/From envelope senders, and `>` quoting for every `^>*From ` body
  line. Archive entries retain the exact source bytes without decoding.
- The client streams to a pending File System Access destination where
  available and commits it only after success. Its portable Blob fallback does
  not trigger a download until every message is complete. Cancellation or a
  missing/corrupt source discards the pending output in either path.
- A domain named in the request must be the caller's own domain or one for
  which SESAME grants domain read authority. FYLO UID/GID access is applied
  again while every page and bucket object is read.

## Conversation Participants

- `GET /threads` and `GET /threads?emailId=...` return normalized unique
  `participants` plus `personalLevel` (`0`, `1`, or `2`) for the authenticated
  mailbox.
- The authenticated browser renders every returned message independently and
  uses each message ID for Reply, Reply All, Forward, full-message, and raw
  source actions. Quoted-content collapse and bounded find-in-conversation are
  local presentation behavior and do not mutate this contract.
- Level `2` identifies a two-person conversation; level `1` identifies mail
  where the mailbox is one of several participants; level `0` suppresses the
  indicator.
- Persisted visible To/Cc/Bcc headers are authoritative when present. The SMTP
  envelope is only a compatibility fallback for messages without recipient
  headers, preventing list mail from being misrepresented as direct mail.

## Compatibility Testing

The same black-box corpus runs against:

1. the current unversioned Tachyon adapter;
2. each Go vertical slice under `/api/v1`;
3. the unversioned Go compatibility alias before Phase 7 removal.

An implementation is not at parity if a test must inspect internal classes,
repositories, or storage layout.
