# db/

This folder is the database layer for CADUCEUS.

## Layout

```text
db/
├── schemas/       # Versioned FYLO/CHEX schemas, developer-owned
└── .collections/  # Local FYLO preview storage, do not hand-edit
```

## schemas/

Each collection uses FYLO's versioned schema layout:

```text
db/schemas/<collection>/
├── manifest.json
└── history/
    └── v1.schema.json
```

The files in `history/` are CHEX regex schemas. Each leaf schema value is a regex string. The `id`, `createdAt`, and `updatedAt` fields are managed by FYLO and omitted from schemas.

## .collections/

`db/.collections/` is owned by FYLO. Do not modify files under this directory by hand.
It is ignored by Git and Dropbox, and the Dockerfile does not copy `db/`, so
local seeded preview data cannot leak into the production image.

For local preview, set `FYLO_ROOT=db`. Recent Fylo versions create and read
collections at `${FYLO_ROOT}/.collections/<collection>/`, with `docs/`,
`index/`, `events/`, and `locks/` directly under each collection.

Use the current standalone runtime configuration:

```env
FYLO_ROOT=db
FYLO_SCHEMA=db/schemas
FYLO_INDEX_BACKEND=local-fs
```

## Attachments

Inbound and outbound attachment metadata live in the `attachments` collection.
Attachment bytes live in Fylo's `attachment-files` bucket under
`${FYLO_ROOT}/.buckets/`. Each metadata record is keyed to its owning email
through `emailId` and references the bucket object through `bucketId`.
Filenames are reduced to a portable leaf name, content types are normalized,
and optional Content-IDs are stored without `cid:` or angle brackets. The
reader resolves only unique safe raster Content-IDs after an authorized bucket
read; attachment metadata never creates a public object URL.
Path-backed records created before the bucket migration remain readable.
If those records used a custom `ATTACHMENT_ROOT`, keep that setting until the
legacy records are no longer needed.

## External provider synchronization

Optional external accounts are Go-owned and split across private FYLO
collections. `provider-accounts` contains only redacted account state;
`provider-credentials` contains versioned encrypted envelopes;
`provider-oauth-transactions` contains owner-bound, single-use state digests
and encrypted PKCE verifiers for Microsoft authorization;
`provider-sync-schedule` is the machine-only restart index; and
`provider-sync-state`, `provider-folders`, `provider-messages`, and
`provider-mutations` hold account-scoped leases, cursors, indexes, and the
journal-first remote action queue. `provider-notifications` is a scheduler-only
durable outbox for incremental unread Inbox imports; initial hydration and
UIDVALIDITY resets never produce alert floods. Owner-facing status reads address one exact
account and mutation. The normal `mail-actions` journal stores only opaque
provider identities and before/after presentation state, allowing one hosted
and provider action to undo or redo durably without copying RFC 822 bodies.
Move mutations retain the ordered UIDPLUS `COPYUID` destination identities;
FYLO updates the indexed message identity in place and keeps its raw bucket so
crash recovery, Undo, and Redo never guess a provider UID.
`provider-submissions` is the durable
authenticated SMTP journal; its bounded RFC 822 source lives separately in the
`provider-submission-files` bucket until delivery or cancellation. Queued rows
carry a monotonic revision. Schema v4 adds a durable `editing` state and
replacement metadata. Tracked delivery stages one bounded bucket per exact
envelope recipient and publishes the complete batch in one CAS. An in-place
scheduled edit keeps the previous batch authoritative until commit; cancellation
or bounded stale-edit recovery deletes staging buckets without losing runnable
mail. A stale scheduler transition cannot publish or deliver an old source, and
a partially delivered batch is never automatically replayed.
Raw imported
RFC 822 bytes live in the `provider-raw-files` bucket and message documents
retain only its opaque bucket identifier. Every owner-facing read repeats the
exact SESAME tenant/principal check below FYLO uid/gid enforcement.

Optional CalDAV synchronization uses `provider-calendar-state`,
`provider-calendars`, `provider-calendar-resources`, and
`provider-calendar-mutations`. One active generation per exact account owner
makes a complete or incremental refresh atomically visible; inactive
generations are removed only after the state pointer moves. The mutation
collection is an exact-owner, idempotent journal for conditional CalDAV writes
and their applied, conflict, or failed outcome. Provider URLs, sync tokens,
ETags, and raw iCalendar remain behind the Go bridge. The browser receives
normalized events, writable collection capabilities, and opaque identities.

Cursor advancement happens after every corresponding idempotent folder,
message, deletion, and raw-bucket mutation. If a checkpoint acknowledgement is
lost, the next run safely repeats stable provider identities. Remote writes are
journaled before IMAP commands and ambiguous acknowledgements enter
`reconciling` rather than being blindly repeated.
SMTP submission likewise records `applying` before the provider command. A
restart or connection loss after DATA becomes terminal `ambiguous` and is never
automatically sent twice. Sent and cancelled source buckets are erased while
their bounded journal records remain available for status and audit.

## Trusted extensions

The `extensions` collection stores exact-mailbox-owner package records. Each
record retains the server-normalized manifest, immutable SHA-256 digest,
bounded source, enabled state, and trust/install timestamps. Collection reads
always include the normalized owner filter and settings projections never
return source. Plugin source is available only through the exact-owner runtime
read; declarative theme records are required to have empty source. Enabling a
theme leaves only one owner theme active.

## Contacts

Personal, found-in-mail, and imported vCard records live in the `contacts`
collection. Contact groups live in `contact-groups`; deleting a group removes
its identifier from member contacts but never deletes those contacts. Both
collections persist the normalized mailbox owner and domain, and repository
reads enforce exact mailbox ownership in addition to FYLO's domain UID/GID
boundary.

## Signatures

Rich composer signatures live in the `signatures` collection. Each row stores
its exact mailbox owner, sanitized HTML, and the exact owned primary or alias
addresses for which it is the default. Repositories combine FYLO UID/GID scope
with mailbox-owner filters; default reassignment patches every affected row in
one FYLO operation so an address cannot retain two defaults.

## Snoozed Conversations

The `snoozed` collection stores one active expiration per mailbox-owned RFC
thread, including the exact message ids moved from Inbox. A signed periodic
worker asks the API to complete due records; only messages that remain in the
Snoozed folder are returned, marked unread, and announced to subscribed
devices. Completed and canceled records remain as bounded idempotency history.
Schema v2 records an optional mailbox-issued client request identity so a
reconnect retry can recover the original result even after the messages have
already moved out of Inbox.

## Send Request Acceptance

The `send-requests` collection is the exact-owner HTTP acceptance journal for
hosted and connected-account delivery. A row binds one caller-selected
idempotency key to the SHA-256 hash of the complete request, records `applying`,
`accepted`, or `ambiguous`, and stores the small accepted response. Reusing a
key for different content fails closed. An accepted retry returns the original
response without touching SMTP; an uncertain post-acceptance outcome is never
automatically repeated.

## Reversible Mail Actions

The `mail-actions` collection is a ten-minute exact-owner transaction journal
for archive, move, trash, read, unread, star, and unstar mutations. Each record
stores only the fields changed for at most 100 messages plus the before/after
values required for recovery, Undo, and Redo. Prepared and in-progress records
are reconciled before the mailbox accepts its next mutation. Unrelated later
field edits are preserved; a later edit to the same field makes the transition
fail closed instead of overwriting the newer state.

## Shared Conversations

The `thread-shares` collection stores exact mailbox ownership, tenant domain,
immutable message membership, explicit expiry, and revocation state for public
conversation links. Only a SHA-256 hash of the 256-bit bearer secret is stored;
the raw link is returned once at creation. Public thread and attachment reads
enter the domain's FYLO group and then repeat exact-owner and membership checks,
so the domain encoded in an untrusted token is only a tenant selector—not
authorization by itself. Creating a replacement revokes the former active link
and deletion is unnecessary for immediate revocation.

## Delivery Tracking

The `emails` v4 schema stores server-issued tracking identity, opt-in flags,
bounded open/click counters, tracked-link metadata, and recipient events.
Reply and forward drafts may also retain a bounded `sourceEmailId` so Send &
Archive can re-authorize the original conversation when the draft is reopened.
Because CHEX leaves are regular-expression strings, links and events are
serialized as validated JSON strings at the FYLO boundary and parsed into
arrays by the repository. Repositories serialize event mutation per tracking
identity and retain at most 100 events per message. The canonical Sent HTML,
raw RFC source, and queue payload do not contain tracking resources; only the
per-recipient SMTP representation does.

## User Preferences

The `users` v5 schema adds `interfaceFont` and `composerFont` catalog
identifiers. The v4-to-v5 upgrader materializes `plex-sans` and `interface`
defaults for existing users. The v6 schema adds the complete `new-mail`,
`hit-send`, `send`, and `confirm` notification-sound map; its upgrader
materializes the fixed local defaults for existing users. Font-family CSS,
local file paths, remote URLs, arbitrary sound events, and arbitrary sound
identifiers are never persisted. The browser maps validated font identifiers
to local/system stacks and synthesizes validated sound identifiers locally.
The v7 schema adds `activeTheme`, `lightTheme`, and `darkTheme` catalog
identifiers. Existing rows upgrade to Automatic with the standard Light and
Dark slots. Theme CSS, file paths, package paths, URLs, and unknown fields are
rejected; only built-in token profiles can cross the persistence boundary.
The v8 schema adds the boolean `githubProfileEnrichment` privacy preference.
Existing rows upgrade to `false`, so upgrades never begin disclosing contact
addresses to GitHub without an explicit user action.
The v9 schema adds the off-by-default local grammar-check toggle, an allowlisted
`auto`, `en-US`, or `en-GB` language, and up to 100 validated ignored rule IDs.
Existing rows upgrade with the feature disabled and no ignored rules.
The v10 schema adds complete per-domain `categoryMappings` for Inbox, Sent,
Drafts, Spam, Archive, and Trash. Existing users receive native same-name
folders. Preference writes are atomic across every assigned domain, reject
duplicate or traversal-capable physical paths, and never bypass a domain
ownership boundary during normalization.
The v11 and v12 schemas add MCP access policy and authoritative unread-badge
scope. The v13 schema adds an exact assigned-domain allowlist for new-mail
alerts, `full`, `sender`, or `private` lock-screen disclosure, and a boolean
notification-action preference. Existing users retain alerts for every
assigned domain with full copy and Mark read/Archive actions enabled; removing
all domains is the explicit all-alerts-off state.
The v14 schema adds the validated keyboard-command override map. The v15 schema
adds the exact-domain custom category catalog, and v16 adds the complete typed
mail workspace, reading, composing, and sending preference profile. The v17
schema adds `interfaceLocale`, initially restricted to `auto`, `en`, `fr`, or
`ar`; v19 widens that closed catalog to every reviewed Mailspring locale plus
CADUCEUS Arabic while continuing to reject arbitrary locale and path values;
existing users upgrade to browser-language negotiation through `auto`.
Arbitrary locale tags never cross the persistence boundary.

The `mail-actions` v2 schema adds optional `clientRequestId` and
`requestFingerprint` fields. Together they make reconnect replay idempotent per
exact owner: an identical request returns its existing durable journal result,
while UUID reuse for different content fails with a conflict.

## Follow-up Reminders

The `follow-up-reminders` collection stores exact mailbox ownership, the Sent
row, canonical RFC thread identity where available, the expected recipients,
and scheduled/due/completed/dismissed state. The background worker queries only
scheduled rows and creates a deterministic `follow-up-<reminder-id>` Inbox
notice, so a crash between message persistence and state transition is
reconciled without creating a second notice. Browser-triggered and worker due
passes are serialized inside the single API writer.

## Inbox Rules

The `inbox-rules` v2 schema adds exact primary-owner and primary-or-alias
mailbox scope, stable order, and stop-processing behavior. New rows always
derive those ownership fields from the authenticated mailbox. Historical v1
rows remain domain-wide until deliberately migrated or recreated, preserving
delivery behavior during upgrade. Preview and production delivery share one
side-effect-free evaluator; only the delivery pipeline executes the resulting
folder, forward, delete, read-state, and star-state actions.

## Authentication State

Authentication data is kept outside user profile documents:

| Collection | Stored data |
| --- | --- |
| `auth-credentials` | Argon2id password hashes and lockout state |
| `auth-transactions` | Short-lived password/passkey and passkey-enrollment challenges |
| `mfa-devices` | Passkey public keys, credential ids and usage metadata |
| `recovery-codes` | Single-use HMAC hashes of recovery codes |
| `auth-audit-events` | Credential and recovery security events |

CADUCEUS never stores biometric material. WebAuthn authenticators verify a
biometric or local PIN on the device and return a signed user-verification
assertion.

The default `CADUCEUS_AUTH_POLICY=passkey-first` preserves normal passkey
sign-in. Set `CADUCEUS_AUTH_POLICY=password-and-passkey` to require a verified
password transaction before passkey login and recent high-assurance
authentication for protected account and routing mutations. Set
`AUTH_PASSWORD_PEPPER` from a
secret manager when an installation requires an additional server-held
password secret.

Authentication collections require durable `FYLO_ROOT` storage. Mount it on a
persistent local disk or volume; never place user credentials, passkeys,
recovery codes, or audit history in an ephemeral temporary directory.

For horizontally scaled deployments, back authentication transactions and
single-use recovery consumption with storage that supports conditional atomic
updates. Fylo is appropriate for a single durable writer, but does not provide
the compare-and-set guarantee needed to rule out simultaneous code redemption
across multiple instances.
