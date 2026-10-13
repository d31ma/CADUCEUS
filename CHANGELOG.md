# Changelog

## Unreleased

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
