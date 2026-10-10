# API Routes Reference

The authoritative v1 route inventory is
[`contracts/http/routes.v1.json`](../contracts/http/routes.v1.json). See
[`API_V1.md`](./API_V1.md) for versioning, authentication, protocol, and
compatibility rules.

The current Tachyon implementation exposes the route paths without the
`/api/v1` prefix. Those paths are migration aliases, not a second API version.

## Route Families

| Family | Purpose | Default authentication |
| --- | --- | --- |
| `/auth/*`, `/mfa/*` | Primary authentication, secondary verification, recovery, and credential management | Public transaction or bearer, per contract |
| `/inbox`, `/search`, `/threads`, `/snooze` | Mailbox reads, interchange, and state transitions | Bearer |
| `/inbox/actions`, `/inbox/actions/{id}/*` | Atomic exact-owner mailbox mutations with bounded undo and redo | Bearer |
| `/thread-shares`, `/thread-shares/{id}` | Create, list, and revoke exact-owner conversation capabilities | Bearer |
| `/thread-shares/public/{token}/*` | Read one expiring shared conversation and its authorized attachments | Public capability |
| `/send`, `/send/scheduled`, `/send/scheduled/{id}`, `/send/undo` | Submission, scheduled delivery management, and undo | Bearer |
| `/activity` | Mailbox-owned delivery events and bounded analytics | Bearer |
| `/drafts`, `/templates`, `/signatures` | Mailbox-owned composer state and reusable content | Bearer |
| `/contacts`, `/contacts/address-books`, `/contacts/{id}/mutations/{mutation_id}` | Exact-owner personal and CardDAV contacts, redacted book selection, and durable remote-write status | Bearer |
| `/categories` | Exact-owner folder/label discovery, creation, and system-role assignments | Bearer |
| `/inbound/webhook`, `/send/tick`, `/reminders/tick` | Internal mail pipeline operations | HMAC |
| `/domains`, `/users` | Tenant and administrative configuration | Bearer plus role/domain authorization |
| `/rules`, `/rules/{id}`, `/rules/order`, `/rules/preview` | Exact-owner mailbox rules, ordering, and side-effect-free preview | Bearer plus mailbox-write authorization |
| `/calendar`, `/presence`, `/notifications` | Collaboration and browser notification features | Bearer |
| `/mcp/config`, `/mcp/token`, `/mcp/audit` | Opt-in MCP scope, capability rotation, and redacted activity | Bearer plus mailbox authorization |
| `/mcp` | Stateless Model Context Protocol tools over HTTP | Dedicated MCP bearer capability |
| `/track/*` | Encrypted delivery-tracking capabilities | Public capability, bounded input |
| `/api/v1/system` | Go runtime identity, build metadata, and readiness | Public |
| `/api/v1/provider-catalog`, `/api/v1/provider-discovery`, `/api/v1/external-accounts/*` | External account onboarding, Microsoft OAuth, lifecycle, sync, and submission | Bearer |
| `/healthz`, `/readyz` | Process and dependency probes | Public |
| operations `/metrics` | Bounded-cardinality Prometheus metrics | Loopback listener only |

Test-only routes are intentionally absent from the public contract and require
both `NODE_ENV=test` and `CADUCEUS_ENABLE_TEST_ROUTES=true`.

## Model Context Protocol

`GET` and `PUT /mcp/config` read and replace the signed-in mailbox owner's MCP
configuration. Access is disabled by default. A complete configuration selects
`read-only`, `read-write`, or `read-write-send`, at least one assigned domain
when enabled, and optional physical-folder exclusions per allowed domain.
Unknown fields, unassigned domains, unsafe paths, and oversized selections fail
closed. A folder exclusion wins over every tool; a conversation containing any
excluded message is hidden as a whole.

`POST /mcp/token` rotates the owner's capability and revokes the previous active
one. The plaintext `mcp1` capability is returned exactly once; FYLO retains only
its SHA-256 hash and lifecycle metadata. `DELETE /mcp/token` revokes it. Tokens
must be sent to `POST /mcp` as `Authorization: Bearer <token>` and must never be
placed in URLs. Authentication derives the FYLO tenant from the signed token
shape before lookup, compares the hash in constant time, then re-loads the
owner's current configuration. Disabling MCP or removing an account therefore
takes effect on the next request.

`POST /mcp` implements the official stateless HTTP transport and supports the
Mailspring tool surface for account/folder discovery, bounded search and reads,
tracking and scheduled-send inspection, draft/reply creation, conversation
state changes, cancellation, sending, and scheduling. Every tool independently
enforces exact owner, allowed domain, excluded folder, and access level. Hosted
CADUCEUS accounts expose both folders and multi-label metadata; `set_labels`
replaces the complete traversal-safe label set only when all selected threads
belong to one allowed account. Requests must
be JSON, are limited to 1 MiB and 600 requests per capability per minute, and a
supplied browser `Origin` must match the request host.

`GET /mcp/audit` returns at most the newest 50 durable events and `DELETE
/mcp/audit` clears them for the exact owner. Events contain only token identity,
tool/category, status, parameter *names*, result count, duration, and time—never
token material, message content, subjects, addresses, or parameter values. The
collection is bounded to 100 events per owner.

`GET /categories` returns one account record per domain assigned to the signed-
in mailbox. Each record contains capabilities, the complete Inbox, Sent,
Drafts, Spam, Archive, and Trash mapping, and exact-owner physical folders and
labels discovered from mail or created by the user. Categories expose bounded
nested-path metadata. `POST /categories` idempotently creates one folder or
label in an assigned domain; traversal, cross-domain writes, kind conflicts,
and oversized catalogs fail closed. Unknown folders have no role.
`categoryMappings` preference writes must
include every assigned domain and role, use distinct traversal-safe paths of at
most 60 characters, and cannot name an unassigned domain. Mailbox APIs expose
the physical path as `folder` and its mapped stable role as `systemFolder`.

## Mail submission notes

`POST /send` accepts an optional `replyTo` array containing exactly one valid
mailbox. The address is normalized, rejected on header-injection characters,
retained through delayed or scheduled submission, emitted as the SMTP
`Reply-To` header, and stored on the Sent copy. Omitting it preserves the
sender address as the effective reply target.

`POST /send` and draft writes also accept an optional `from` mailbox. It must
exactly match the authenticated user's primary address, one of their aliases,
or the mailbox address of their active send-capable external account; sharing a
hosted or provider domain is not authorization to impersonate another mailbox.
The selected address is retained in drafts, scheduled work, the Sent copy, and
the SMTP `From` header, while storage ownership and failure notifications stay
bound to the authenticated primary mailbox. Every `POST /send` accepts an
optional bounded `idempotencyKey`. CADUCEUS binds it to the complete request in
an exact-owner FYLO acceptance journal, returns the original accepted response
on retry, rejects key reuse for different content, and never automatically
repeats an ambiguous outcome. Connected-account delivery additionally journals
raw source before it is runnable,
and supports undo, scheduled cancellation, and revision-checked in-place
schedule editing through `PUT /send/scheduled/{id}`. A queued edit atomically
replaces the provider MIME bucket before updating the local management and Sent
projections; a delivery that has begun returns `409`. Connected sends support
the same self-hosted open and link tracking as hosted mail for one or many
envelope recipients. Recipient-specific provider MIME is staged in FYLO and
becomes runnable only after one atomic batch commit; an interrupted edit keeps
the former batch authoritative and is automatically recovered. Send & Archive is
supported for connected messages when the IMAP server returns UIDPLUS
`COPYUID`; delayed Undo Send restores the source through the durable provider
action journal.

`POST /send` accepts optional `openTracking` and `linkTracking` booleans. Both
default to `false`, require HTML when enabled, and are rejected when supplied
with the wrong type. The server—not the caller—creates the opaque tracking
identity. The Sent row and retained MIME source remain clean; the SMTP adapter
adds a recipient-specific encrypted open capability and rewrites only
non-quoted links for the corresponding recipient.

`GET /activity` returns exact-owner local metrics and events for the signed-in
mailbox and its owned aliases. `filter` is `all`, `open`, or `click`; `range`
is `7d`, `30d`, `90d`, or `all`; `limit` is bounded. Results include aggregate
sent/received/reply/open/click metrics, daily totals, per-subject performance,
an unseen count after optional `since`, and at most 100 newest recipient
events drawn from the newest 500 tracked messages.

`GET /track/open?t=...` and `GET /track/click?t=...` accept only encrypted,
authenticated, expiring capabilities issued by CADUCEUS. Click capabilities
bind the recipient, message identity, stored link index, and exact destination;
invalid or altered destinations never become an open redirect. These endpoints
are intentionally bearer-less because mail clients load them, but they do not
accept caller-supplied message IDs or redirect URLs.

Follow-up reminders are scheduled through `POST /send`'s optional `remindAt`
or `POST /reminders` for an existing exact-owner Sent message. Incoming RFC
thread replies from a named recipient complete a pending reminder. The signed
internal `POST /reminders/tick` scans all mailboxes independently of browser
traffic and returns bounded `due`, `surfaced`, `reconciled`, and `failed`
counters. Deterministic notice IDs make a retry after a crash idempotent.

`/signatures` stores bounded, sanitized rich HTML for the exact mailbox owner.
A signature may claim default use for exact owned primary or alias addresses;
each address has at most one default, and reassignment atomically removes the
claim from the previous signature. Foreign records are hidden as not found.

Inbound messages may expose normalized `headerFrom`, `replyTo`, and
`securityWarnings` metadata. `reply_to_mismatch` means the declared Reply-To
mailbox differs from the visible From mailbox; clients must present the
warning before directing a reply to that address.

`GET /inbox`, `GET /search`, and `GET /threads` aggregate hosted mail with the
signed-in owner's indexed external accounts. A request may include either
`domain=<hosted-domain>` or `account=<external-account-id>`, never both. The
account filter is an opaque exact account identity—not a provider domain—and
is reauthorized by the Go owner boundary before any folder, message, source,
attachment, or mutation lookup. Equal RFC message IDs from two provider
accounts therefore remain separate conversations. An accepted external
mutation's idempotency key returns its original journal record even if
background sync has since advanced the account revision; reusing the key with
a different payload remains a conflict.

`GET /inbox/stream` uses an opaque composite SSE cursor with independent
hosted and connected-account positions. Each source reads its oldest unseen
bounded slice, and a temporary provider outage leaves only the provider cursor
unchanged, so hosted progress cannot skip the provider gap. Stream payloads
contain the redacted mailbox projection only—never credentials, raw source,
bucket IDs, bodies, or attachment bytes.

The IMAP worker negotiates capabilities after authentication. A
QRESYNC-capable mailbox advances its durable `HIGHESTMODSEQ` only in the same
FYLO commit that applies all bounded flag and `VANISHED` deltas. A
CONDSTORE-only mailbox scans bounded UID ranges against a fixed modification
sequence target and advances that target only after the complete range is
checkpointed; explicit UID presence windows still detect expunges. Servers
that omit or reject both extensions use the existing bounded UID/flag
reconciliation. Every path remains fenced by UIDVALIDITY.

IMAP IDLE is an acceleration path, not a second synchronization writer. A
bounded per-account watcher receives mailbox change notifications and only
wakes the durable schedule. The existing account worker then acquires the
normal lease, reloads credentials and checkpoint state, and commits changes.
Unsupported servers continue through the one-minute poll.

Before any remote mutation, the IMAP connector verifies that the UIDVALIDITY
embedded in every stable message identity still matches the selected folder.
Delete marks and expunges only the exact UID via UIDPLUS. A server that rejects
`UID EXPUNGE` has CADUCEUS's `\\Deleted` flag rolled back and the mutation fails
closed; CADUCEUS never substitutes mailbox-wide `EXPUNGE`.

The signed loopback boundary keeps authentication calls at a 1 MiB request and
response limit. Account calls use a separate bounded envelope sized for the
application's 25 MiB MIME-source policy (47 MiB decoded request payload and a
64 MiB response/frame ceiling), so large source and attachment reads do not
silently inherit the smaller authentication limit.

`PATCH /api/v1/external-accounts/{id}` with `action=update` compare-and-swaps
the optional `label`, six-digit hexadecimal `color`, and bounded integer
`order`. It cannot change credentials, endpoints, capabilities, ownership, or
lifecycle state. The sidebar uses this operation for color selection and
adjacent account ordering; a stale revision returns `409` and triggers a fresh
owner-scoped account read.

`POST /api/v1/external-accounts/oauth/microsoft/begin` creates a ten-minute,
single-use, exact-owner OAuth transaction and returns the Microsoft
authorization URL. FYLO retains only the SHA-256 state digest and an encrypted
PKCE verifier. `POST .../complete` consumes that transaction, exchanges the
authorization code server-side, validates IMAP Full Access and SMTP AUTH, and
then creates or revision-safely reconnects the account. Provider access and
refresh tokens are never returned to the browser. For a shared mailbox,
`authIdentity` is the delegate while `mailboxAddress` and
`sharedMailboxAddress` are the shared address; the latter is used as the
XOAUTH2 mailbox identity and SMTP envelope/header sender.

The equivalent `POST /api/v1/external-accounts/oauth/google/begin` and
`.../complete` routes use the same owner-bound transaction, PKCE, encrypted
grant, validation, refresh, reconnect, and redaction rules for Gmail and Google
Workspace. Google accounts must connect their exact sign-in mailbox; delegated
Microsoft-style shared mailbox identities are rejected. Both OAuth flows also
request their provider's delegated calendar and contact consent. The Go worker
synchronizes and conditionally mutates those calendars and address books
through the same owner-scoped boundaries used by CalDAV and CardDAV accounts.
Grants issued before these scopes were introduced require one explicit
reconnect.

`GET /inbox/:id` and its mutable-field `PUT` response may include a derived
`githubUrl` only when an exact `github.com` sender supplied an explicit
View-on-GitHub anchor whose target is credential-free HTTPS on `github.com` or
`www.github.com`. The value is never persisted from message input. Clients must
revalidate it before opening a new no-opener, no-referrer browsing context.

Validated mailing-list metadata is exposed as optional `unsubscribeUrl`,
`unsubscribeMailto`, `unsubscribeBodyUrl`, and `unsubscribeOneClick` fields.
Only credential-free HTTPS endpoints and single-recipient mailto targets are
retained. A body URL is a lower-priority fallback detected outside quoted
content when no header method exists. Clients must require an explicit user
action and must never trigger an unsubscribe request while displaying or
prefetching a message.

`GET /threads?emailId=<id>` resolves the authorized conversation containing a
specific message across folders. It returns an empty array when the message is
outside the caller's domain scope and includes attachment summaries, never raw
attachment bytes, for conversation rendering and printing. Conversation
membership follows tenant-scoped `Message-ID`, `In-Reply-To`, and `References`
edges; only historical rows without retained RFC metadata use the legacy
normalized-subject fallback. The browser consumes this response as a complete
per-message reader: message controls operate on the selected message ID,
quoted-content disclosures are derived after sanitization, and bounded
find-in-conversation highlighting never changes or persists message content.

Thread responses include normalized unique `participants` and `personalLevel`.

`POST /thread-shares` accepts an exact-owner `emailId` and an
`expiresInDays` value of `1`, `7`, or `30`. It returns the secret link once;
FYLO persists only its SHA-256 hash, immutable message membership, owner,
domain, expiry, and revocation state. Creating another link for the same thread
revokes the previous active link. `GET /thread-shares` returns management
metadata without any reusable secret, and `DELETE /thread-shares/{id}` revokes
only an exact-owner record.

The bearer-less `/thread-shares/public/{token}` family parses the tenant domain
from the bounded capability, enters only that FYLO group, verifies the secret
hash in constant time, and re-authorizes every stored message against the exact
mailbox owner. Attachment reads repeat capability, immutable-membership, email,
and owner checks before reading bucket bytes. Invalid, altered, expired, and
revoked links all return the same not-found shape. Public HTML is rendered in
a sandbox after scripts, forms, remote resources, and tracking elements are
removed; Bcc, tracking metadata, raw source, storage identifiers, and mailbox
ownership are never presented.
`personalLevel` is `2` when the authenticated mailbox and one other address are
the complete participant set, `1` when the mailbox is one of three or more
participants, and `0` otherwise. Visible To/Cc/Bcc headers take precedence over
the SMTP envelope so mailing-list delivery does not become a false direct-mail
indicator.

Mailbox reads are scoped below caller-controlled filters by the exact canonical
`ownerEmail`, not merely by hosted domain. An explicit stored owner is
authoritative even when another local mailbox appears in To/Cc/Bcc. Historical
rows without ownership use narrow folder-specific address fallbacks and
ambiguous rows fail closed. `/inbox`, `/search`, `/threads`, detail, source,
attachments, and `/inbox/stream` therefore share the same mailbox boundary.
The reader's participant sidebar composes authorized `/contacts?q=<address>`
and `/search?q=from:<address>` results locally. Optional public GitHub
enrichment is isolated behind `GET /github/profile?email=<address>` and the
disabled-by-default `githubProfileEnrichment` preference. After opt-in, the
route still accepts only an exact personal contact or participant in mail owned
by that mailbox; it calls only GitHub's fixed public API origin and returns a
bounded sanitized profile plus at most five repositories. Local context does
not depend on GitHub availability and no token is required.

`GET /inbox/:id/source` returns the authorized message's exact retained RFC
822 bytes as a base64 payload with its SHA-256 digest, byte size, MIME type, and
safe `.eml` filename. It applies the same verified domain scope as message and
attachment reads; source is never embedded as executable HTML.

`POST /inbox/:id/images` is the authenticated remote-image privacy boundary.
Every requested URL must be present in that exact owned message. The server
rate-limits requests, resolves and pins only wholly public DNS answers,
revalidates redirects, restricts ports and raster formats, verifies magic
bytes, and returns bounded base64 data. The sandbox never receives a network
URL.

`GET /inbox/export/rfc822?folder=<name>&domain=<domain>` or
`GET /inbox/export/rfc822?folder=<name>&account=<external-account-id>` streams a versioned,
bounded sequence of exact message sources for client-side mboxrd or individual-
EML ZIP64 assembly. The client exposes both folder export formats, reports live
message/byte progress, and can cancel without installing a partial file. EML
archives require one concrete domain; mbox can combine domains after separately
authorizing each one. Connected-account exports resolve an exact owner-scoped
provider folder, page its Go-owned index, and read each raw source through the
same owner boundary. Every paged FYLO metadata and bucket read retains the
authorized UID/GID context.

`POST /inbox/import/eml` imports one exact RFC 822 source into an explicitly
authorized mailbox and non-draft folder. The 50 MiB per-file bound is checked
before MIME parsing. Source, attachments, and the derived message row commit as
one recoverable unit; a failure removes staged FYLO bucket objects. Imports do
not run SMTP routing, forwarding, webhooks, inbox rules, or notifications.

`POST /inbox/actions` applies one atomic action to one through 100 exact-owner
hosted or connected-account messages. Its allowlisted patch can move messages
or change their read/starred state; portable labels remain hosted-only. The
durable FYLO journal records only affected fields plus opaque provider mutation
identities, returns an opaque action identifier, and expires after ten minutes. `POST
/inbox/actions/:id/undo` and `/redo` are mailbox-owner-only, idempotent at their
target state, recover interrupted transitions before accepting another action,
preserve later edits to unrelated fields, and return `409` instead of
overwriting a later conflicting edit. Connected-account transitions query
`GET /api/v1/external-accounts/:id/mutations/:mutation_id` through the exact
owner Go boundary before queuing an inverse; each later redo uses a new durable
generation while a crash retry reuses the same idempotency key.

`POST /drafts` idempotently creates a mailbox-owned draft using `clientId`.
`GET`, `PUT`, and `DELETE /drafts/:id` load, compare-and-swap, and discard it.
Updates require the last acknowledged integer `version`; a stale writer receives
`409` with the current version and update time. Draft responses alone include
attachment bytes so an interrupted composer can recover exactly. Ordinary
mailbox, thread, search, source, and attachment reads enforce the exact draft
owner in addition to the caller's authorized domains.

`GET /inbox/:id/attachments/:attachment_id` returns base64 bytes only after
exact mailbox authorization and email-to-attachment binding. Filenames are
portable server-normalized leaf names. Browser previews remain authenticated
client behavior; there is intentionally no public or bearer-less attachment
URL.

`POST /snooze` resolves the complete authorized RFC conversation containing
`emailId`, moves its current Inbox messages to Snoozed, and records one durable
expiration. Its optional `clientRequestId` returns the original exact-owner
result when a browser reconnects after the messages have already moved.
`DELETE /snooze/:id` provides the bounded undo action. The signed
`POST /snooze/tick` system route is driven by an independent non-overlapping
worker; due messages still in Snoozed return to Inbox unread, the record is
completed idempotently, and subscribed devices receive a deep-linked alert.

`GET /users/me/preferences` returns `remoteImagePolicy` (`block` or `allow`)
and an exact, normalized `trustedImageSenders` list alongside the other user
preferences. It also returns catalog-backed `interfaceFont` and `composerFont`
identifiers, the `interfaceLocale` choice (`auto`, `en`, `fr`, or `ar`), and a
complete `notificationSounds` map for `new-mail`, `hit-send`, `send`, and
`confirm`. `auto` negotiates only bundled catalogs from the browser's ordered
language list and otherwise falls back to English. Each sound is one of `none`,
`chime`, `bell`, `pop`, or `pulse`. `themePreferences` stores a catalog-backed `activeTheme` plus separate
`lightTheme` and `darkTheme` choices used by Automatic mode. `PUT
/users/me/preferences` accepts these fields independently. The boolean
`githubProfileEnrichment` preference defaults to `false`; enabling it is the
explicit consent boundary for sending a selected owned participant address to
GitHub's public API. `grammarCheckEnabled`, `grammarCheckLanguage`, and
`grammarCheckDisabledRules` persist the off-by-default standalone checker,
its `auto`, `en-US`, or `en-GB` language choice, and at most 100 ignored rule
identifiers. The route otherwise
rejects arbitrary font names, locale tags, URLs, CSS, sound events, and sound identifiers,
theme package names, and unknown theme fields, and accepts the remote-content fields independently. The route
caps the trusted list at 500 addresses, and never uses image trust to expand
mailbox authorization. Reader clients must keep spam blocked automatically and
must remove recognized tracking pixels even when ordinary images are allowed.

The same preferences response includes `newMailNotificationDomains`, an exact
subset of assigned hosted domains; `newMailNotificationExternalAccounts`, an
exact subset of owner-authorized connected-account IDs;
`newMailNotificationPrivacy` (`full`, `sender`, or `private`); and the boolean
`newMailNotificationActions`. Empty account lists disable new-mail Web Push for
their respective account class. Provider sync records incremental unread Inbox
arrivals in a scheduler-owned FYLO outbox before committing its cursor. A
signed internal worker leases and settles those records with bounded retry;
initial hydration and UIDVALIDITY resets are suppressed. Action clicks carry no credential in
the worker and are applied only after the authenticated client re-authorizes
the exact message through the durable mailbox-action route.

### Trusted extensions

`POST /extensions/inspect` accepts `{ bundle: { manifest, source } }`, performs
strict normalization and size/allowlist validation, and returns a redacted
projection plus the immutable SHA-256 digest. It does not persist or activate
the package. `POST /extensions` repeats that inspection and installs only when
`trustedDigest` is an exact match, preventing a package from changing between
review and trust. New packages are disabled.

`GET /extensions` lists only the authenticated mailbox owner's redacted
packages and never returns source. `GET /extensions/:id` is the owner-scoped
browser-runtime read and includes source for that one package. `PUT
/extensions/:id` accepts only `{ enabled: boolean }`; enabling a theme disables
every other theme owned by the mailbox. `DELETE /extensions/:id` removes only
an exact-owner package. Foreign identifiers return 404.

Plugin manifests target CADUCEUS engine 26, request only the documented
`compose:read`, `compose:write`, `mail:metadata`, and `ui:notify` permissions,
and contribute bounded commands to `app-toolbar`, `composer-toolbar`, or
`message-toolbar`. Theme manifests request no permissions, contain no source,
and declare only allowlisted hexadecimal color tokens.

`POST /grammar/check` accepts at most 20,000 plain-text characters from an
authenticated mailbox and returns a bounded LanguageTool-compatible match
shape with exact offsets, messages, categories, and up to five replacements.
It is rate-limited per mailbox and runs entirely inside CADUCEUS; no text is
persisted or sent to an external service. The browser leaves spelling to the
native checker, discards stale results, defers during IME composition, and
renders grammar ranges through CSS Custom Highlights so annotations never
enter draft or delivered HTML.
