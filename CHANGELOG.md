# Changelog

## Unreleased

- Started Mesail, the rewrite of CADUCEUS as one messenger and email app
  (ADR 0004). Go for the Tac frontend and the Yon backend on TACHYON
  26.46.02, one Tachyon app per service, FYLO 26.45.07 with one root per
  service, Keycloak (OIDC) instead of SESAME. Added the spec, domain model,
  API contracts, implementation slices, the shared `kit/` packages, the app
  skeletons with health routes and a fail-closed identity provider, blackbox
  acceptance tests and `scripts/gates.sh`. Removed the CADUCEUS JavaScript
  server and client, the Go monolith, SESAME and their tooling (kept in git
  history at 4ef82f72).

### Slice R1: realtime and presence

- Added `edge/realtime`, the connection gateway: a small Go program that
  holds WebSockets (`/api/v1/realtime/socket`) and event streams
  (`/api/v1/realtime/stream`), the local stand-in for an AWS API Gateway
  WebSocket API. It calls the `realtime` app's `connect`, `message` and
  `disconnect` routes and offers the `@connections` management API.
- Added `kit/realtime` and the `realtime` routes: token in the first socket
  message, re-checked at least every 60 seconds (`auth.accepted`, then
  `auth.expired` and close 4401), single-use stream tickets, fan-out of
  `message.*`, `reaction.changed`, `read.moved`, presence and typing within
  one domain, presence states with `offline` after 60 seconds without a
  connection, and a sweep job for expired connections and presence.

### Slice F2: web conversations

- Sidebar (`components/sidebar/nav`, `SidebarService`): a "New message"
  button, then Channels, Direct messages and Email sections with unread and
  mention counts, private, archived and email-address markers, and loading,
  empty and error states. Data is kept for the current domain only and is
  cleared when the domain changes (chat never crosses a domain).
- Timeline (`components/conversation/*`, `ConversationService`): day and
  "New" dividers, grouped messages, mentions, edit (read on submit), delete
  with a confirm dialog and the "email cannot be recalled" warning,
  reactions by name (12), attachments, delivery lines with words and icons
  ("Seen by 3 people", "Bounced for …"), "Load older", newest message first
  in view, local times from the browser time zone. On touch screens the
  actions open from a "More actions" button.
- Email display: only the server's rich-text blocks are rendered (no HTML),
  the sender check (verified, sent on this server, could not verify),
  remote images hidden until the sender is allowed (then opened as a link,
  because the page CSP allows no remote images), quoted text behind a
  toggle, and "Get the original" for long emails.
- Live updates (`RealtimeService`): answers `hello`, `reauth` and
  `auth.expired` with a fresh token, and refreshes the sidebar and the open
  conversation on `message.*`, `reaction.changed` and `read.moved`; new
  messages are announced to screen readers.
- Requests renew the token once after a `401`. The composer (F3) is opened
  with `SendTarget` and the timeline refreshes after `SendCompleted`.
- TACHYON 26.46.02 does not render after an `@OnEvent @InBackground`
  handler; each such handler ends by publishing `ConversationRefreshed`,
  which a plain handler takes.

### Slice P1: Lambda images, the Floci stack and the native run

- `scripts/native/run-all.sh`: the path every slice gate uses. It starts
  every Yon app with `ty start` on macOS (FYLO roots in the ATLAS test
  image), serves the Tac apps, puts the Mesail router in front on one
  origin, runs the blackbox tests and stops everything. `--up` and
  `--down` keep it running for manual work.
- `deploy/router`: the Mesail router, a standard-library Go program. Native
  mode proxies `/api/v1/<service>/` to each `ty start`; Lambda mode invokes
  `mesail-<service>` with API Gateway HTTP API v2 events (FitNest's design).
  `/idp/` goes to Keycloak, everything else is the `web` and `web-admin`
  bundles (from folders or S3) with security headers.
- `Dockerfile` target `lambda`: one arm64 AWS Lambda container image per Yon
  app (Ubuntu 24.04, Go 1.27.3, `ty` 26.46.02, `fylo` 26.45.07, all checksum
  pinned, AWS Lambda Web Adapter 0.9.1). It runs on Lambda's read-only
  filesystem as a non-root user, and each FYLO owner keeps its root at
  `/mnt/data/<service>`. Target `router` builds the router image.
- `deploy/floci/stack.json`, `docker-compose.yml` and `scripts/floci/`:
  the Floci 2.4.0 integration run in Colima with Keycloak under `/idp`,
  one function per service (reserved concurrency 1 and one EFS access point
  for every FYLO owner), static hosting in S3, seeded realms `example.com`,
  `example.org`, `partner.test` and `mesail-operators`, and test tokens.
  Guide: `deploy/floci/README.md`.

### Slice I2: events and outbox

- `kit/events`: one payload type per event, the producer and consumer
  catalog, the envelope factory, strict payload reading and the consumer
  `Receiver` (checks the `Service` caller, the envelope, the listed producer
  and the domain; drops repeats by `domainId` and `eventId`).
- `kit/outbox`: pending events stored inside the changed FYLO document, a
  relay that pushes them over HTTP to the subscribers in `application.json`
  with backoff, dead letters and replay, and test helpers for other slices.
  It works with reserved concurrency 1 per Lambda.
- `events` and `outbox` are in `kit.list` of directory, channels,
  messaging, mail-in, mail-out, search, realtime and notify, with synced
  copies. Guide: `docs/mesail/EVENTS.md`.
- Fix: `outbox.SettingsReader` reads `outbox` from under the `yon` key, as
  a running handler sees `YON_APPLICATION_JSON` (before, it found no
  settings outside tests, so nothing was relayed). New `FromEnvironment()`.
  Retry keys are lowercase (`basedelayseconds`, `maxdelayseconds`,
  `maxattempts`).

### Slice M1: inbound mail

- Added `edge/smtp`, the long-running inbound SMTP listener (the MX). It
  checks each RCPT TO with `directory` (550 5.1.1 for an unknown address,
  550 5.7.1 for a domain that is not hosted), offers STARTTLS when a
  certificate is configured, enforces size, recipient, line and connection
  limits, checks SPF, DKIM and DMARC (550 5.7.1 for a DMARC `reject`
  failure), and answers 250 only after every recipient domain's raw copy
  is written durably and `mail-in` stored it. The delivery pipeline has no
  socket code, so a managed inbound service can use it in production.
- Added the `mail-in` route `POST /api/v1/mail-in/messages`: it stores one
  `InboundRaw` per recipient domain, answers the same id for a retried
  message, and queues one `email.received` event.
- Added `kit/mime` (header and MIME parser, fuzz tested) and `kit/mailauth`
  (SPF, DKIM and DMARC checks with the DMARC DNS tree walk, fuzz tested).
- The edge writes raw copies through `kit/blobs` (S3, or a local folder
  in development) and maps only `directory`'s two 404 codes; any other
  answer gets `451` so the sender retries.
- `mail-in` keeps its workflow in `kit/mailin` (rule 8b), stages
  `email.received` with `kit/outbox`, and serves
  `GET /api/v1/mail-in/messages/{inboundId}/content` (the email converted
  once into typed blocks) and `.../original` (a 5-minute signed link to
  the raw message), both for the caller's own domain only. An internal
  hand-off needs a hosted, active sender domain and is copied into the
  recipient's domain.
- Added `kit/richtext`: HTML email, plain text and chat Markdown become
  typed blocks with a strict allowlist and limits; fuzz tests show no
  markup or other link scheme gets through.

### Slice I1: identity and token check

- `kit/oidc` checks Keycloak access tokens for real in every service: it
  fetches and caches each realm's key set (JWKS), checks RS256 and ES256
  signatures, the issuer pattern (one realm per mail domain), the
  `mesail-api` audience, `azp`, `typ` and expiry with 30 seconds of leeway,
  and still refuses every token when its `oidc` settings are missing.
  `oidc.SigningRealm` lets every app test with real tokens and no network.
- The identity service signs people in server to server through Keycloak
  (`POST`/`DELETE /api/v1/identity/session`, `POST .../session/refresh`)
  with an AES-GCM encrypted `sessionKey`, and answers `GET /identity/me`.
- `oidc.ServiceTokensFactory{}.Configured().Authorization(realm)` gives a
  service a cached `mesail-service` token (client credentials) for calling
  another service. Guide: `docs/mesail/SERVICE-TOKENS.md`. The Keycloak
  token endpoint is the shared setting `oidc.keycloak`.
- Account settings in identity: `POST /api/v1/identity/password`,
  `GET`/`DELETE /api/v1/identity/totp`, `POST .../totp/setup` (secret,
  `otpauth://` URI, server-drawn QR code and a sealed `setupKey`) and
  `POST .../totp/confirm`, through the Keycloak admin API as the client
  `mesail-account`. Session routes and account routes share one controller.
- identity follows Keycloak 26.8.2 as tested by mesail-keycloak: a locked
  account is found through the brute-force status of the user (Keycloak
  answers it like a wrong password), and the 2-step secret is stored with
  the stock admin API, so no custom Keycloak endpoint is needed.

### Mesail D1: directory

- Added the `directory` service: hosted domains (an Operator from the
  `mesail-operators` realm adds one, which creates its Keycloak realm
  through the admin API as `mesail-directory`; verify checks MX, SPF, DKIM
  and DMARC over DNS over HTTPS and activates a pending domain; remove
  deletes the realm and the domain's collections), members remembered from
  the caller's token, aliases, the `Service` address and domain lookups,
  member search inside the caller's domain, and
  `POST /api/v1/directory/recipients/resolve`. A member of the caller's
  domain is `member` (chat), any address of another hosted domain is
  `otherDomain` (email only, no name or id), anything else is `external`.
- A changed member stages `member.upserted` with `kit/outbox` in the member
  document and pushes it right after the write; `POST
  /api/v1/directory/outbox/relay`, `GET .../outbox/dead` and `POST
  .../outbox/dead/requeue` (`Allow(Service)`) sweep and manage it. Pushes
  wait for slice I1's service tokens.
- `kit/recipients` gained address and domain-name checks and the recipient
  classifier, which decides the kind from the caller's domain.
- For other slices: the address lookup answers `404 domain_not_hosted`
  for a domain that is not hosted or not active and `404 not_found` for an
  unknown address (M1); `GET /api/v1/directory/members/{memberId}` and
  `POST`/`DELETE /api/v1/directory/channel-addresses` for channels (C2).
- Adding a domain now creates its realm from mesail-keycloak's
  `deploy/keycloak/realm/domain-realm.json` (a copy embedded in the
  directory, checked by a test), with the `mesail-identity`,
  `mesail-service` and `mesail-account` secrets from directory's `.env`;
  an empty secret refuses the call. A `@Schedule` worker triggers
  `POST /api/v1/directory/outbox/relay` every 60 s with a
  `mesail-operators` service token.
- The outbox sweep reads only member documents with `outboxWaiting: true`
  (and the dead-letter view `outboxHasDead: true`), set with the counts in
  every write. The directory tests reuse fixed domains per test unit and
  reset them, so a test FYLO root no longer grows.
- Realm checks follow Keycloak 26.8.2's rights for `mesail-directory`:
  existence is read from the public discovery URL; a seeded realm is
  adopted when the create answers 409 (or 403 while discovery finds it);
  removing a domain whose realm the directory did not create answers
  `403 forbidden` and keeps the domain. The outbox sweep reads only due
  documents and stops after 20 seconds.

### Slice A1: attachments and blob store

- Added the `attachments` service and the `kit/blobs` blob store port.
  Attachment bytes live outside FYLO, under a key that starts with the
  owning domain, in a local folder (all-in-one image and native dev) or in
  an S3-compatible bucket (Floci locally, AWS later). Uploads and downloads
  use short-lived signed links. Completing an upload checks the size, the
  SHA-256 and the real content type. Download links go only to members who
  may read the message that carries the file, and never to another domain.
- `kit/blobs` also writes and copies blobs for the SMTP edge and the mail
  hand-off, and its `LocalHandler` serves the local adapter's signed links.
- `kit/attachments` holds the upload, complete and download workflow in
  plain Go, so the Yon role files stay thin enough for TACHYON's
  cohesion check (`ty bundle` enforces TY1314).
- `GET /api/v1/attachments/{attachmentId}/meta` lets a `Service` caller
  (`messaging`, at send time) read an attachment's name, type, size,
  SHA-256 and state in its own domain; another domain or an unknown id
  answers `404`, any other caller `403`.

### Slice M3: messaging

- The `messaging` app stores conversations, participants (with each
  member's read position), messages (split by month, with their delivery
  rows and rich-text blocks), reactions, mentions, email thread keys and a
  member cache, all in FYLO collections named per domain with
  `kit/collections`. Every lookup is scoped to the caller's domain; data of
  another domain answers `404`.
- One composer: `POST /api/v1/messaging/messages` re-resolves every
  recipient through `directory` at send time and refuses a stale kind
  (`409 recipients_changed`), chat to an email-only kind
  (`422 chat_not_allowed`) and email to a channel (`400`). It reuses a
  chat-only direct conversation by its member set, starts a new email
  conversation when any delivery is email, keeps Bcc out of participants
  and headers, threads email replies (`In-Reply-To`, `References`), and
  publishes `message.created` and one `email.send.requested` per email
  recipient through the outbox. A retry with the same `clientMessageId`
  returns the first message. At most 20 recipients and 25 storage writes
  per send.
- Lists, threads (one level), edit, delete with a placeholder, reaction
  names, read positions with unread and mention counts, delivery status
  (`read` from the reader's position), the `/source` route for `mail-out`
  and the `/original` download for received email.
- Consumes `channel.*` (the channel's conversation and members),
  `member.upserted`, `email.delivery.changed` (status never moves back)
  and `email.received`: a reply joins its conversation and thread by
  `Message-ID`, only inside the same domain; a new email to several
  members of one domain is one shared conversation.
- `kit/messaging` holds the fixed list of reaction names.
- Body blocks come from a plain-text renderer until `kit/richtext` lands.
- A send reads each attachment from `attachments` (`/meta`) before any
  write: an unknown or not-ready file answers `400` (`attachmentIds`), more
  than 50 MiB together answers `413 too_large`. The message keeps each
  file's name, media type and size, answers them in `attachments[]`, and
  puts the file names in `message.created` for search.

### Slice M2: outbound mail

- `kit/compose` builds RFC 5322 messages: `Message-ID`, `In-Reply-To` and
  `References` for threading (at most 20 references, the first one kept),
  RFC 2047 and RFC 2231 encoding, text, HTML and attachments, CRLF line
  ends, and no `Bcc` header ever. It refuses line breaks in header values.
- `kit/dkim` signs each message with its own domain's key (RSA-SHA256 or
  Ed25519-SHA256, relaxed/relaxed, `from` signed twice). Keys come from the
  secret store under `dkim/<domain>/<selector>`, never from documents or
  logs, and a From outside the key's domain is refused. Two selectors can
  live side by side, so keys rotate without lost mail.
- `kit/smtpclient` delivers direct to MX (implicit and null MX) or through
  a relay (STARTTLS, AUTH PLAIN only over TLS, port 465), through an HTTP
  CONNECT tunnel when the app runs behind TACHYON's outgoing guard, and
  sorts replies into temporary and permanent. `kit/smtpclient/sink` is a
  local SMTP server for tests, so no test sends real mail.
- The `mail-out` app takes `email.send.requested` from messaging and
  notify, keeps one queue item per recipient (deduplicated by event id and
  idempotency key), builds and signs the message, and delivers it: to
  `mail-in` over HTTP when the recipient's domain is hosted on the same
  install (never SMTP to itself), otherwise to the relay or direct to MX.
  Temporary failures retry with doubling delays up to one hour, until 5
  days; hard bounces add the address to the sender domain's own suppression
  list. Every change publishes `email.delivery.changed` through the outbox.
  The delivery job and the relay are `Allow(Service)` routes that a
  scheduled worker or an EventBridge schedule triggers.

### Slice S1: search

- Added the `search` service: `GET /api/v1/search` finds messages by words of
  the body, email subject, sender and file names, with conversation, author,
  date, attachment and email filters, newest first, paged by cursor. Results
  come only from the caller's domain and only from conversations the caller
  can read at query time (checked with `messaging` and `channels` using the
  caller's token; when that check fails, the answer is `503` with no hits).
  `POST /api/v1/search/events` indexes `message.created`, `message.edited`
  and `message.deleted` through the `kit/events` receiver. The index keeps one
  FYLO collection per domain and month, named by `kit/collections`.
- The workflow is the plain-Go package `kit/search` (rule 8b); the Yon role
  files are thin. `kit/textindex` holds word splitting, prefix matching,
  snippets and cursors.

### Slice N1: notifications

- `apps/notify` turns `message.created` into notifications (AC-PN-2): a
  mention, a direct message (`email` when it is an email) or a reply in a
  thread the member wrote in. Only members who can read the message get
  one, never the author, and a muted conversation keeps only mentions. One
  call writes at most 20; the rest wait in a fan-out record for the job.
- Members list and read their notifications (newest first, paged), save
  push subscriptions, read the VAPID public key and set the summary delay
  and muted conversations. Everything is per domain: another member's or
  domain's notification answers `404`.
- Web Push (AC-PN-3) in the new package `kit/webpush`: VAPID (RFC 8292),
  `aes128gcm` encryption (RFC 8291, checked against its Appendix A), an
  allowlist of push service hosts against SSRF, and a `Target` seam for
  native push later. Nothing is pushed during `dnd` or while the member is
  `active`; the text never holds message content; gone subscriptions are
  deleted. The VAPID keys come from `.env` (secret store).
- The email summary (AC-PN-4) goes out once per batch of unread
  notifications after the member's delay as `email.send.requested` from
  `notify` (now a second listed producer in `kit/events`); mail-out reads
  its body from `GET /api/v1/notify/messages/{id}/source`.
- Rules (recipients, push decisions, summary text, paging) are in the
  plain-Go package `kit/notify`. `POST /api/v1/notify/jobs/run` (Service,
  install-wide) does fan-out, push and summary work in bounded calls, and a
  `@Schedule` worker triggers it and the outbox relay.

### Slice C2: channels

- `apps/channels` lists, creates, joins, leaves, invites to and updates
  (topic, archive, email address) the channels of the caller's domain,
  stored per domain in FYLO (`channels-<domain key>`,
  `channel-members-<domain key>`). Private channels and channels of other
  domains answer `404`; an invite of a member of another domain is refused.
- The workflow is the plain-Go package `kit/channels` (rule 8b): the two
  controllers make one call into it, through `DataSource.Command` for FYLO.
  `go test` covers it fully; the Yon tests drive every route with real
  signed tokens and a real FYLO test root.
- Each change stages its `channel.*` event in the changed document and
  pushes it right after the write, with a service token from the realm of
  the event's domain; `/api/v1/channels/outbox/*` (Service) sweeps, counts
  waiting documents, lists dead letters and retries them, and a `@Schedule`
  worker calls the sweep route every minute under `ty start` with a
  `mesail-operators` token.
- Invite checks and channel email addresses call directory
  (`GET /api/v1/directory/members/{memberId}`,
  `POST`/`DELETE /api/v1/directory/channel-addresses`) with the caller's
  token, so directory decides the domain and keeps addresses unique.

- Completed account-synchronized interface localization across 20 closed,
  persisted locale choices. A reproducible, checksum-pinned SMaLL-100 pipeline
  now catalogs 755 audited static interface literals for 19 non-English
  locales, converts Taiwan Traditional Chinese with pinned OpenCC dictionaries,
  preserves English for rejected output, and excludes user/mail content from
  DOM localization. The full route matrix now qualifies both LTR and RTL at
  phone and desktop sizes, including mirrored, keyboard-accessible resizers.

- Added a session-bound AES-GCM encrypted browser mailbox mirror for the newest
  500 messages. Authenticated idle slices hydrate full bodies and bounded
  attachments; inbox loading, structured search, reader rendering, and
  attachment access fall back locally during an API outage. No token or mail
  plaintext is persisted outside the encrypted vault, which is destroyed on
  sign-out or unauthenticated boot.

- Completed Gmail's one-message/many-label synchronization model. The IMAP
  connector now requires `X-GM-EXT-1`, uses `X-GM-MSGID` as the canonical
  identity, projects `X-GM-LABELS` without duplicating FYLO raw buckets, and
  performs crash-reconcilable label moves. Provider qualification now covers
  Gmail, Microsoft/shared mailboxes, the password-provider presets, and custom
  IMAP/SMTP through deterministic protocol transcripts.

- Added capability-negotiated IMAP CONDSTORE and QRESYNC synchronization.
  QRESYNC applies bounded flag and VANISHED deltas atomically with the durable
  mailbox modification sequence; CONDSTORE-only servers checkpoint bounded UID
  range sweeps and retain explicit presence windows for expunge detection.
  Servers that omit or reject the extensions continue through the existing
  UIDVALIDITY-safe reconciliation path.
- Added bounded IMAP IDLE wakeups for connected accounts. One supervised
  watcher per eligible account holds only short-lived server-side credentials,
  turns `EXISTS`, `RECENT`, `EXPUNGE`, and `FETCH` notifications into a durable
  scheduler wake, and reconnects after a bounded 25-minute wait. Unsupported
  or failed IDLE connections fall back to the existing minute poll without
  changing mailbox state outside the lease-owning sync worker.
- Completed provider-native contact synchronization for Gmail/Google Workspace
  and Microsoft 365 accounts. Google People and Microsoft Graph contacts now
  use bounded incremental or full-refresh synchronization, exact-owner FYLO
  projections, writable address-book discovery, conditional updates/deletes,
  and idempotent create reconciliation. OAuth consent includes the required
  contact scopes; existing OAuth accounts require one reconnect.
- Completed integrated-calendar provider parity with native Google Calendar
  API v3 and Microsoft Graph v1 adapters. OAuth consent now includes bounded
  calendar discovery and write scopes; the Go worker projects recurrence,
  exceptions, cancellations, attendees, reminders, colors, and read-only state
  through the exact-owner FYLO boundary and performs durable conditional writes
  with deterministic retry identities. Existing OAuth accounts require one
  reconnect to grant the new scopes.
- Added a session-bound AES-GCM IndexedDB vault for ordered offline mailbox
  actions, draft autosave/recovery, Send and Send Later, snooze, and calendar
  writes, with serialized Background Sync replay and no persisted bearer token
  or plaintext message body.
- Added durable FYLO send-request acceptance journals plus idempotent snooze and
  local-calendar request identities so reconnect retries cannot duplicate an
  accepted operation after a lost HTTP response.

## Unreleased — Phase 1 Go Runtime Foundation

- Completed native mail-transport qualification with real ESMTP/STARTTLS/MIME
  interoperability, recipient-scoped partial-delivery retry and bounce
  handling, permanent-address suppression, null-MX and transient-DNS behavior,
  signed queue recovery with secret-free health, and a CI-gated deterministic
  72-hour queue soak that proves accepted recipients are never duplicated.
- Added revision-checked in-place editing for connected-account scheduled
  sends. FYLO stages recipient-specific MIME buckets behind an `editing` CAS,
  keeps the former batch authoritative until atomic commit, restores abandoned
  edits after a bounded lease, and retains the canonical scheduled/Sent
  identity. Self-hosted open and link tracking now works for one or many
  connected-account recipients without contaminating the retained Sent source;
  partial batch delivery becomes explicitly ambiguous and is never replayed.
- Made connected-account moves crash-safe by persisting IMAP UIDPLUS `COPYUID`
  destination identities before FYLO projection. Provider Move Undo/Redo and
  connected Send & Archive now follow the latest UID, while delayed Undo Send
  restores the archived source. Scheduler rescheduling also preserves a
  concurrent run-now wake instead of delaying interactive mutations by a full
  poll interval.
- Completed the trusted plugin/theme extension system with exact-owner FYLO
  lifecycle, two-step SHA-256 inspection and explicit trust, closed permission
  and toolbar-slot allowlists, opaque network-disabled worker sandboxes,
  declarative code-free theme tokens, responsive sideload settings, and unit,
  route, and real-browser teardown coverage.
- Completed CardDAV contact parity with exact-owner address-book selection,
  idempotent remote creation, server-generated resource identities,
  `If-None-Match` and ETag preconditions, ambiguous-response reconciliation,
  durable mutation status, explicit conflict choices, and vCard 3/4 plus
  grouped Apple-property interoperability coverage.
- Added the first account-synchronized interface-localization slice with
  English, French, and Arabic catalogs; strict FYLO v17 locale preferences;
  automatic browser-language negotiation; plural interpolation; document
  language/direction state; authenticated shell and settings translations;
  RTL-responsive verification; and locale-aware date, time, number, byte, and
  analytics formatting across the primary mail workflows.
- Completed connected-mailbox folder lifecycle parity with owner-resolved,
  crash-reconcilable IMAP create, rename, and delete mutations; protected
  system folders; modified UTF-7 Unicode names; accessible sidebar dialogs;
  and exact connected-folder EML/mbox export through the existing streaming
  source-frame pipeline.
- Added Mailspring-compatible date-only all-day calendar events with exclusive
  ends, RFC 5545 DATE recurrence/exception/RSVP support, multi-day rendering,
  all-day editing, DST-safe date-space movement, local-time-preserving timed
  moves, and an explicit agenda all-day drop row. Timed recurring occurrences
  retain their recurrence identity instead of being implicitly converted.
  Selected events now remain visible across view changes, and occurrence-
  relative whole-series edits correctly apply both move and resize changes to
  timed and all-day masters.
- Made non-action toast surfaces pointer-transparent so durable status messages
  cannot block calendar or mailbox controls beneath them; toast actions remain
  directly interactive.
- Added delimiter-aware, collapsible connected-account folder trees with
  account-scoped persisted disclosure state, non-selectable IMAP grouping,
  orphan-safe paths, accessible controls, and unit plus browser coverage.
- Hardened rich-text paste handling with active-content removal, safe links,
  theme-neutral foreground stripping that preserves intentional formatting,
  and stable Windows/legacy newline normalization.
- Completed installable-PWA integration with validated `mailto:` protocol,
  Web Share, and `.eml` file handlers; navigate-existing launch behavior; a
  bounded credential-free reconnect queue; Background Sync wakeups; and FYLO
  v2 request UUID deduplication for lost-response-safe mailbox actions. The
  full manifest, launch, import, failure, and reconnect contract passes in
  Chromium, Firefox, and WebKit.
- Completed per-account new-mail notifications with typed FYLO preferences,
  full/sender/private lock-screen modes, existing sound-cue integration,
  retry deduplication, optional Mark read and Archive buttons, and a
  credential-free service-worker bridge that re-authorizes actions through the
  durable exact-owner mailbox boundary. Added responsive settings plus worker,
  route, payload, and real-Chromium lifecycle coverage.
- Completed Mailspring-compatible undo/redo for single and bulk archive, move,
  trash, read, unread, star, and unstar actions with an exact-owner FYLO journal,
  all-or-nothing authorization, bounded recovery, conflict-safe field patches,
  accessible responsive selection controls, bulk drag/drop, browser-native
  offscreen row rendering, roving focus, message/conversation context menus,
  and real-browser archive/Undo/Redo coverage.
- Added fully self-hosted translation fallback with an immutable int8
  SMaLL-100 model, authenticated checksum-verified chunk delivery, a matching
  Asyncify ONNX Runtime, bounded local preferences/result caching, protected
  HTML translation, a full local detector for all 80 model/Mailspring shared
  languages, and real Spanish-to-English and English-to-Finnish inference
  qualification in Chrome, Firefox, and WebKit. No message or draft text is
  sent to a hosted translation service.
- Completed the 93-language Mailspring translation catalogue with two pinned,
  lazy OPUS Marian ONNX directions. Missing-language pairs pivot locally
  through English, every model file remains checksum-locked behind the
  authenticated asset route, and real inference covers both OPUS directions
  in Chrome, Firefox, and WebKit.
- Rebased the production FYLO mail-store root from
  `/data/caduceus/fylo` to `/data/caduceus`, including the incoming-file
  staging path, Compose bind mount, image layout, operator documentation, and
  Phase 2 topology gate. SESAME keeps its independent `/data/sesame` deployment
  and FYLO writer root.
- Completed Mailspring-compatible category mapping with exact-owner folder
  discovery, typed per-domain FYLO preferences, strict atomic validation,
  stable logical roles across delivery and every mailbox operation, responsive
  searchable settings, and route/unit/real-browser coverage.
- Upgraded Tachyon to `26.33.02`, adopting its exclusively client-rendered Tac
  structure so companion-backed conditions and loops bundle without the prior
  `TY1303` server-render boundary.
- Added a traceable 74-requirement Mailspring parity ledger and a strict
  completion audit covering all 52 upstream built-in packages plus 22
  platform capabilities.
- Completed Mailspring-compatible mailbox rules for every mailbox owner with
  exact primary/alias ownership, richer predicates and state actions,
  deterministic reordering, stop-processing, production-parity dry-run
  previews, responsive management, and real-browser lifecycle coverage.
- Completed self-hosted open and link tracking with explicit composer opt-in,
  encrypted per-recipient capabilities, clean retained Sent originals,
  redirect validation, mailbox-owned event details, unseen Activity counts,
  and bounded daily and subject analytics.
- Added a real-browser tracking flow that protects composed rich HTML across
  Tachyon option rerenders and bootstraps the Activity dashboard correctly
  after client-side navigation.
- Completed no-reply follow-up reminders with an accessible preset/custom
  composer chooser, RFC-thread reply resolution, exact-owner list/open/dismiss
  behavior, an HMAC-driven supervised all-mailbox worker, deterministic crash
  reconciliation, secret-free measurements, and real-browser lifecycle tests.
- Completed Mailspring's View on GitHub action for exact `github.com`
  notification senders, with a validated reader-toolbar action, `mod+g`
  shortcut, and fail-closed sender and target lookalike handling.
- Completed Mailspring's optional GitHub contact card with a disabled-by-
  default privacy preference, fixed-origin bounded public API adapter,
  exact-owner contact/participant authorization, sanitized profile and top-
  repository output, graceful offline/quota fallback, and responsive browser
  coverage. It requires no GitHub token or hosted configuration.
- Completed Mailspring's optional composer grammar checker with a bounded,
  rate-limited local English engine, LanguageTool-compatible issue results,
  persisted toggle/language/ignored-rule preferences, stale/IME-safe debounce,
  non-serializing CSS highlights, accessible replacements, and responsive
  browser coverage. Draft text never leaves CADUCEUS.
- Completed Mailspring-compatible conversation sharing with self-hosted,
  256-bit secret links, hash-only FYLO persistence, one-, seven-, and
  thirty-day expiry, exact-owner immutable snapshots, replacement rotation,
  immediate revocation, attachment reauthorization, protected public HTML,
  copy/open controls, and responsive real-browser coverage. No conversation is
  uploaded to Mailspring or another hosted service.
- Completed Mailspring's conversation message list with independent expansion,
  per-message Reply/Reply All/Forward/Open/`.eml` controls, sanitized
  quoted-content disclosures, and bounded find-in-conversation highlighting
  plus next/previous keyboard traversal.
- Completed Mailspring's participant profile and true mode-switch behavior with
  a persistent, responsive Contact sidebar backed only by mailbox-owned local
  contacts and related conversations. Exact-owner enforcement now covers all
  message folders and live inbox replay, with same-domain isolation tests; the
  standalone control retains native `aria-pressed` semantics while DuVay issue
  #9 tracks equivalent web-component forwarding.
- Completed Mailspring-compatible attachment handling with FYLO bucket
  lifecycle coverage, portable untrusted filenames, normalized Content-IDs,
  authenticated inline raster rendering, inert image/text/PDF previews, exact
  downloads, and shared picker/paste/drag-drop composer limits.
- Completed configurable authoritative unread app badges with a persisted
  all-hosted-domains or focused-domain scope, responsive settings, exact-owner
  validation, a privacy-aware tab-title fallback, server-counted Web Push
  payloads, background notification deep links, and a Tachyon post-bundle
  worker hook.
- Completed Mailspring-compatible composer templates with exact-owner CRUD,
  searchable insertion, draft-to-template creation, responsive management,
  Tab/Shift+Tab editable blanks, empty-area warnings, safe signature/quote
  preservation, and delivery-time removal of authoring markup.
- Completed Mailspring-compatible per-address signatures with exact-owner FYLO
  persistence, safe rich-clipboard editing and preview, primary/alias defaults,
  composer switching and removal, draft From recovery, reply/forward placement
  before quoted mail, and exact sender authorization through SMTP delivery.
- Completed Mailspring-compatible conversation snoozing with all six upstream
  presets plus a custom time, RFC thread identity, exact-owner FYLO state,
  navigation-safe Undo, due labels, an independent idempotent return worker,
  unread resurfacing, and deep-linked Web Push notification.
- Added an optional, injection-safe Reply-To composer field across immediate,
  undo-delayed, and scheduled delivery. Inbound mail now preserves normalized
  From/Reply-To metadata, warns when the reply target differs from the visible
  sender, and directs reply actions to the declared Reply-To mailbox.
- Added safe RFC 2369/RFC 8058 List-Unsubscribe handling with validated HTTPS
  and mailto targets, explicit one-click intent, confirmed reader actions, and
  no automatic request while merely viewing a message.
- Added privacy-safe conversation printing and browser PDF export across
  authorized mailbox folders, with chronological messages, participants,
  attachment summaries, and tracking-free standalone output.
- Added privacy-first HTML email rendering: remote images and CSS fetches are
  blocked before iframe insertion, per-message and exact-sender consent are
  available, preferences persist in FYLO's versioned user schema, spam fails
  closed, and tracking pixels remain removed after ordinary images are shown.
- Completed the reader content-security boundary with an authenticated,
  exact-message, SSRF-resistant raster image proxy that never exposes remote
  URLs to the iframe, plus visible Unicode/punycode and displayed-host mismatch
  warnings with a responsive actual-destination confirmation.
- Converted the client shell into an explicit Tachyon component used by every
  route, replaced browser-invalid decorator syntax with Tachyon lifecycle
  metadata and native events, and made DuVay readiness deterministic.
- Upgraded the qualified runtime pair to FYLO `26.33.02` and SESAME
  `26.33.02`, including the tagged SESAME Go client, exact platform digests,
  container and CI pins, FYLO's explicit child-environment shim support, and
  the restored authenticated-runtime gate.
- Added the Go 1.26.5 `caduceus` operator binary with `version`, `doctor`,
  `serve`, and read-only `migrate` preflight commands.
- Added strict shared configuration, locked FYLO digest verification, immutable
  build metadata, request IDs, JSON logs, liveness, readiness, loopback
  Prometheus metrics, and bounded graceful shutdown.
- Added the versioned `/api/v1/system` contract and retained `/system` and
  `/health` compatibility aliases.
- Added a manifested Tachyon-to-Go static bundle handoff with per-file and
  aggregate SHA-256 verification.
- Added Go unit and binary black-box tests, race/vet/staticcheck CI, an 80%
  coverage ratchet, and a CRAP score ceiling of 15.
- Kept the legacy production/mail/authentication runtime unchanged pending the
  later vertical-slice cutovers.

## v26.20.1 (2026-05-12) — Mailspring-Inspired Enterprise UI

### UI Overhaul
- **3-panel mail layout**: sidebar (200px) | email list (360px) | reading pane (flex)
- **Mailspring-aligned CSS variables** (`--ms-*` mapped to Material Design tokens)
- **Dark mode**: light/dark/auto themes with animated sun/moon toggle
- **Collapsible sidebar sections**: Mailboxes, Folders, System groups with localStorage persistence
- **Email row density**: comfortable/compact/default toggle
- **Hover quick actions**: star, archive, trash icons appear on email row hover
- **Drag-and-drop**: drag emails to sidebar folders to move them
- **Swipe gestures**: mobile email rows support swipe for quick actions

### Keyboard Shortcuts (23 bindings)
- Mousetrap integration with Mailspring-style keymap
- Navigation: `j/k`, `g+i/d/s/a/x/t`, `/` (search)
- Email actions: `s` (star), `e` (archive), `#` (trash), `r` (reply), `f` (forward)
- Composer: `mod+enter` (send), `esc` (discard)
- See `docs/KEYBOARD.md` for full reference

### New Mail Features
- **Undo-send**: 10-second delay window with countdown toast
- **Send later**: schedule email delivery for a future time
- **Send & archive**: single-click send + archive
- **Snooze**: hide emails until later (presets: today, tomorrow, weekend, custom)
- **Inline reply**: reply directly from the reading pane
- **Rich text composer**: bold, italic, lists, links toolbar
- **Templates**: save and load reusable email templates
- **Signatures**: per-domain email signatures with auto-append
- **Threading**: conversation grouping with expand/collapse
- **Read receipts**: tracking pixel for open detection
- **Link tracking**: redirect proxy for click tracking

### Full-Text Search
- Gmail-like query syntax: `from:`, `to:`, `subject:`, `has:attachment`, `is:unread`, `before:`, `after:`
- Quote support: `from:"John Doe"`
- `GET /search?q=...`

### New Pages
- `/folder/drafts`, `/folder/sent`, `/folder/archive`, `/folder/spam`, `/folder/trash`
- `/scheduled` — view scheduled messages

### Infrastructure
- 14 new server routes (see `docs/API-ROUTES.md`)
- 6 new repository modules (outbox, snooze, scheduled, templates, threads, tracking)
- Search query parser service
- Audit screenshot and DOM audit scripts
- Blackbox tests made non-blocking in PR CI (`continue-on-error: true`)
