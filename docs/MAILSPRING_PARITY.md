# Mailspring Feature Parity

This is the execution contract for the goal of implementing every current
Mailspring feature in CADUCEUS. The machine-readable source of truth is
[`mailspring-parity.json`](mailspring-parity.json); this document explains how
to use it.

## Upstream baseline

- Repository: `https://github.com/Foundry376/Mailspring`
- Branch: `master`
- Audited commit: `d93a0d7f43049dd81e1ff0cdf9957915dd5d62b6`
- Latest audited release: `1.23.0`
- Audited on: 2026-08-22

The two commits since the previous baseline contain a Slate-specific unpaired
UTF-16 backspace crash fix and a `tar` dependency security override; they add
no packages or user-facing feature requirements to the ledger. CADUCEUS uses
the browser's native `contenteditable` editing path and does not depend on
`tar`, so neither upstream implementation patch applies directly.

The ledger contains all 52 directories under Mailspring's
`app/internal_packages/` plus 22 cross-cutting capabilities found in its
README, changelog, sync engine, menus, keymaps, and locale system. The latter
cover behavior that is not represented by one package, including provider
sync, shared mailboxes, offline operation, RFC threading, EML/mbox/vCard
interchange, recurrence, accessibility, plugins, localization, render modes,
and security boundaries.

The 2026-08-21 refresh inspected all 11 commits after the prior baseline.
They added Hebrew catalog verification, defensive alias parsing, theme-safe
rich-text paste behavior, and calendar all-day, drag, resize, and selection
semantics. Existing strict alias validation and the localization gap cover the
first two; the paste behavior and date-only all-day/drag semantics are
implemented below. Native Google Calendar and Microsoft Graph adapters now
complete the remaining `main-calendar` provider synchronization gap.

The initial audit found 74 requirements. The current ledger has all 74
implemented, with 0 partial and 0 missing. `implemented` is deliberately
strict. A similarly named route or UI control is only accepted when the
complete user behavior and observable automated tests exist.

## Product adaptation rule

Parity means preserving the user capability, not copying Electron-specific
implementation details. CADUCEUS remains a self-hosted multi-domain mail
server with a Tachyon web/PWA client and a Go production backend.

Examples:

- Mailspring's system-tray unread count maps to PWA Badging API support plus a
  tested fallback, not an Electron tray process.
- Mailspring's local MCP server maps to an opt-in, loopback or administrator-
  controlled MCP surface with SESAME authorization, mailbox scoping, and an
  audit trail.
- External Gmail, IMAP, and Microsoft connectors are optional additions; they
  must not make CADUCEUS depend on a hosted service or weaken its standalone
  SMTP operation.
- Pro features are implemented locally without a subscription dependency.

## Status and evidence

| Status | Required evidence |
| --- | --- |
| `implemented` | End-to-end production behavior, local source paths, and at least one observable automated test |
| `partial` | Real working behavior exists, but the ledger names the remaining user-visible or operational gap |
| `missing` | No production behavior satisfies the requirement yet |

Completion requires every package and platform capability to be
`implemented`, every cited path to exist, the completion audit to pass, and a
final browser/runtime verification against the ledger.

```sh
# Structural audit; expected to pass throughout implementation.
bun run audit:mailspring-parity

# Completion gate; required to pass with all 74 requirements implemented.
bun run audit:mailspring-parity:complete
```

## Delivery order

Work proceeds by vertical slice so each increment crosses storage, service,
API, UI, accessibility, and tests when those layers are relevant.

1. Mail model and interchange: RFC threading, raw source retention, durable
   drafts, contacts, List-Unsubscribe, EML, mbox, and vCard.
2. Rendering and safety: remote-image policy, tracking-pixel protection,
   render modes, print, phishing/link warnings, and PII-safe screenshot mode.
3. Productivity: complete snooze processing, undoable actions, templates,
   signatures, grammar, translation, reminders, analytics, profiles, and
   personal-level indicators.
4. Calendar and account connectivity: recurrence, provider calendar sync,
   IMAP/Gmail/Microsoft connectors, category mapping, and shared mailboxes.
5. Platform experience: full preferences, notifications, badges, offline
   writes, accessibility, keyboard coverage, themes, localization, trusted
   extensions, and MCP.
6. Completion audit: run every requirement's observable test, browser and
   responsive matrices, security gates, migration/rollback checks, and the
   strict parity completion command.

## Implemented in the current parity program

- The PWA now maintains a session-bound AES-GCM encrypted mirror of the newest
  500 mailbox records. Online list refreshes replace the local summary index;
  idle authenticated slices hydrate missing full bodies and bounded attachment
  payloads without persisting bearer tokens. Inbox loading, structured
  full-text search, reader rendering, and attachment preview/download fall
  back to the encrypted mirror when the API is unavailable. Sign-out and an
  unauthenticated shell boot destroy both the key and ciphertext. Unit tests
  cover bounds and query semantics, and a real-browser outage test proves
  body-only search plus attachment reads while verifying that IndexedDB does
  not contain the subject, body, or attachment plaintext.

- Connected-mail synchronization now qualifies Gmail/Google Workspace,
  Microsoft/Outlook/shared mailboxes, the five password-provider presets, and
  custom IMAP/SMTP at deterministic protocol boundaries. Gmail requires
  `X-GM-EXT-1`, imports All Mail, Spam, and Trash as canonical physical
  sources, stores exactly one FYLO record and raw bucket per `X-GM-MSGID`, and
  projects every current `X-GM-LABELS` membership. Native label moves preserve
  that identity and reconcile after ambiguous restarts; deleting a stale label
  cannot delete a message still present under another label.
- Native hosted-domain transport now qualifies real ESMTP/STARTTLS/MIME wire
  behavior, direct MX and null-MX resolution, DKIM-capable delivery, standard
  submission/inbound listeners, provider-owned SMTP connectors, and durable
  recipient-scoped retry/bounce semantics. Signed queue operations expose
  bounded health without message content, while the CI transport gate advances
  1,152 continuous submissions through a deterministic 72-hour state-machine
  soak and asserts zero duplicate accepted deliveries.

- Per-mailbox extension packages now have a two-step sideload flow: the server
  normalizes and hashes an immutable manifest/source bundle, and installation
  requires the user to trust that exact SHA-256 digest. Exact-owner FYLO
  records drive enable, disable, single-active-theme, and removal lifecycle.
  Plugins execute only in opaque-origin Web Workers hosted by sandboxed
  iframes with network, storage, forms, navigation, and remote assets denied;
  every host context and result is filtered through declared permissions and
  bounded command slots. Themes contain no code or CSS and can set only a
  fixed hexadecimal color-token catalog. Unit, FYLO-backed route, and
  real-Chromium tests cover validation, owner isolation, responsive settings,
  runtime commands, CSP, and teardown.
- Interface localization now has one closed 20-choice preference synchronized
  through FYLO v19, browser-language negotiation, fallback lookup,
  interpolation and plural rules, document `lang`/`dir` state, and locale-aware
  date, time, number, byte, and analytics formatting. A reproducible pinned
  SMaLL-100 pipeline inventories 755 static interface literals and publishes
  19 non-English catalogs with reviewed core mail commands, Taiwan Traditional
  Chinese conversion, checksum provenance, strict rejection, and explicit
  English fallback. The DOM localizer excludes sender, recipient, subject,
  message, attachment, contact, calendar-event, activity, and extension data.
  Real authenticated Chromium coverage verifies persistence, content safety,
  French LTR, Arabic RTL, Hebrew behavior across all routes, 320-pixel and
  desktop containment, logical CSS edges, and mirrored pointer/keyboard column
  resizing.
- Unified mailbox navigation now keeps a hosted domain and every connected
  provider account as distinct identities, even when their addresses share a
  domain. Owner-scoped provider folders flow from the Go FYLO boundary into
  desktop sidebar sections and the responsive mailbox selector; inbox, search,
  and RFC conversation routes accept one mutually exclusive `domain` or
  `account` scope. External source, body, attachment, quick and bulk
  read/star/move/delete actions, and thread presentation reuse the normal
  reader contracts. Mutation replay returns the original journal entry before
  checking a revision advanced by background sync, while changed payloads
  still conflict. The account bridge keeps auth frames at 1 MiB but admits
  bounded large MIME frames for account operations; its route fixture proves
  source, attachment, and thread reads above 1 MiB. Unit, real IMAP/SMTP route,
  and browser tests cover same-domain isolation, provider folder routing,
  responsive selection, attachment reads, bulk actions, and idempotent remote
  actions. Destructive IMAP mutations verify the message's UIDVALIDITY epoch,
  delete through exact `UID EXPUNGE`, and roll back `\\Deleted` rather than
  issuing an unsafe mailbox-wide expunge when UIDPLUS is unavailable. Durable
  mixed hosted/provider read, star, and move undo and redo use one owner action
  journal with provider-conflict refresh recovery. IMAP MOVE requires UIDPLUS
  `COPYUID`; its destination identity is persisted before FYLO projection,
  reconciled after crashes, and followed by later Undo and Redo operations.
  Connected-account Send & Archive uses the same journal, and Undo Send restores
  the source message through its latest provider identity.
  Incremental unread provider Inbox imports
  enter a scheduler-owned FYLO notification outbox before checkpoint advance;
  a signed worker applies the same exact-account, privacy, action, and sound
  policy as hosted Web Push. Initial hydration and UIDVALIDITY reset floods are
  suppressed. The live stream uses an opaque two-source cursor and replays the
  oldest unseen bounded slice independently for hosted and connected mail.
- Connected-account sidebar headers now expose the account's durable color and
  keyboard-accessible adjacent ordering controls. Changes use the authenticated
  Go owner boundary, exact-owner lookup, optimistic account revision, and a
  compensating rollback if the second half of an order swap fails. Route and
  browser tests cover persistence, hostile color rejection, and the rendered
  control surface. Provider delimiters project the flat remote catalog into a
  collapsible, account-scoped hierarchy without fabricating missing parents;
  non-selectable IMAP roots remain visible as groups. Disclosure state survives
  reloads, uses stable provider identifiers, and has unit plus browser coverage.
  Connected folders now add accessible create-child, rename, and guarded
  deletion actions backed by the durable Go mutation journal. The command
  boundary resolves only owner-scoped stable identifiers, protects system
  folders, persists provider paths for crash reconciliation, and sends Unicode
  mailbox names as modified UTF-7. Connected views export exact per-folder RFC
  822 sources to the existing mboxrd or EML ZIP64 streams. Focused protocol,
  FYLO, route, unit, and browser tests cover these actions. The account-sidebar
  package is complete; transport-specific optimizations remain independently
  tracked by the provider-sync platform capability.
- Mail preferences now have a typed FYLO v16 profile and atomic API validation
  for workspace layout and scale, clock and unread-count policy, delayed/manual
  read state, detailed headers and names, message width and color mode,
  Backspace and touch-swipe behavior, conversation order, rich/plain composing,
  spellcheck language, default From/reply/send actions, sender-specific CC/BCC,
  undo-send duration, attachment previews, snooze return position, and per-domain
  found-in-mail learning. Unit, route, migration, and real-browser coverage proves
  persistence, reload behavior, sandboxed light/dark rendering, keyboard/touch
  actions, composer payloads, attachment safety, and exact-owner server effects.
- Message event cards now select `.ics`, `text/calendar`, legacy `.vcs`, and
  `application/ics` payloads with upstream-compatible precedence, suppress
  same-size provider duplicates without hiding unrelated files, and keep the
  selected invitation downloadable through the exact-owner attachment route.
  Cards present timed and timezone-stable all-day ranges, location, organizer,
  recurrence and cancellation state, guarded RSVP actions, and REQUEST/REPLY
  participant states including delegated and completed responses. Unit,
  FYLO-backed import/route, and Chromium, Firefox, and WebKit interaction tests
  cover the full message-card journey.
- Recurring calendar events now use bounded RFC 5545 rules with stable series
  and occurrence identities, explicit recurrence and exception dates,
  timezone-aware expansion across daylight-saving transitions, and imported
  `RECURRENCE-ID` exceptions and cancellations. The calendar exposes recurring
  indicators and explicit instance-versus-series edits and deletions, enforces
  read-only and past-event boundaries server-side, and discloses when the rest
  of a series remains. Unit, FYLO-backed route/import, and Chromium, Firefox,
  and WebKit interaction tests cover creation, editing, cancellation,
  navigation, accessibility, and focus restoration.
- Calendar cells now offer a focused quick-event dialog with one-step creation
  or escalation into the complete editor. Owned event chips support native
  pointer drag/drop between month and agenda days while preserving local time,
  duration, recurrence-instance identity, read-only rules, and the existing
  keyboard move/resize alternative. Chromium, Firefox, and WebKit exercise
  both interaction paths.
- All-day events are now stored and exchanged as RFC 5545 civil dates with an
  exclusive end instead of timezone-sensitive midnight timestamps. Multi-day
  spans render on every covered date; form, ICS recurrence/exception/reply,
  month drag, and keyboard paths preserve that representation through DST.
  Month-cell timed drags retain wall-clock time, while only the explicit
  all-day row converts a non-recurring timed event. Unit, FYLO-backed route,
  and real-Chromium interaction coverage exercises creation, editing,
  recurrence identity, multi-day movement, and timed-to-all-day conversion.
- Calendar selection is ID-stable across refreshes and view changes. Switching
  to a narrower view moves the cursor to the selected occurrence and brings its
  chip into view. Entire-series edits made from any occurrence retain that
  occurrence in the editor and apply its calendar-day or instant move delta
  plus new duration back to the master. Route and browser tests cover timed and
  all-day whole-series move/resize behavior.
- Optional IMAP accounts can now add an HTTPS CalDAV endpoint during guided
  setup. The Go connector performs bounded principal, calendar-home, and
  collection discovery, then uses complete queries or RFC 6578 sync-token
  deltas to publish one atomic, exact-owner FYLO snapshot. The calendar merges
  recurring remote events into every responsive view, identifies their account
  and collection, and removes duplicate imported invitation copies without
  exposing raw iCalendar bytes to the browser. Exact-owner, idempotent mutation
  journals now drive conditional CalDAV create, update, series/instance edit,
  and delete operations with ETag conflict detection and ambiguous-response
  reconciliation. Protocol, connector, FYLO, bridge, route, unit, and browser
  tests cover authentication, unsafe endpoints, token expiry, owner isolation,
  stable identities, writes, and secret handling. Gmail and Workspace now add
  bounded Calendar API v3 collection discovery, per-calendar incremental
  tokens, sparse recurring-cancellation recovery, and deterministic event
  identities. Microsoft accounts now add bounded Graph v1 collection/event
  pagination, delegated shared-mailbox routing, recurrence conversion, and
  deterministic transaction IDs. Both use the same exact-owner FYLO projection
  and durable conditional mutation boundary, including read-only collection
  enforcement.
- Archive, move, trash, read, unread, star, and unstar now share one durable
  exact-owner action journal for single and bulk mutations across hosted and
  connected accounts. The service authorizes the complete mixed set before
  writing, uses deterministic generation-scoped provider idempotency keys,
  completes interrupted transitions on retry, preserves unrelated later edits,
  rejects same-field or rejected-provider conflicts, and exposes consistent
  Undo/Redo/Refresh feedback from list, reader, keyboard, and drag/drop
  interactions. Provider action authorization uses metadata-only exact-owner
  reads instead of downloading RFC 822 bodies. The responsive list supplies accessible per-message
  and per-conversation selection plus a horizontally scrollable bulk toolbar.
  Its resizable responsive columns use browser-native offscreen row rendering,
  one roving tab stop with Arrow/Home/End navigation, and bounded accessible
  message/conversation context menus backed by the same reversible actions.
- Consented remote email images now cross an authenticated exact-message proxy
  that validates and pins wholly public DNS, rechecks redirects, limits rate,
  ports, formats, signatures, bytes, and time, and returns only inert raster
  data to a network-disabled sandbox. Disguised visible hosts and Unicode or
  punycode destinations are marked in-message and require a responsive actual-
  destination confirmation before opening.
- Users can choose whether the installed-app and tab-title unread badge counts
  every hosted mailbox or one hosted domain. The exact-owner preference is
  normalized and persisted in FYLO, applied immediately to foreground
  snapshots, reused by authoritative Web Push counts, retained across user
  updates and reloads, and exposed through a responsive settings control.
- New-mail Web Push policy is persisted per hosted account. Users can disable
  individual accounts, choose full, sender-only, or private lock-screen copy,
  mute the platform alert through the existing new-mail sound preference, and
  disable notification actions. Delivery retries replace the same tagged
  notification. Allow-listed Mark read and Archive buttons cross no bearer
  token into the worker; they are re-authorized by the authenticated mailbox
  action service in a focused or newly opened app client.
- The installable PWA owns `mailto:` links, shared text/URLs, and `.eml` files.
  Manifest launch input is bounded and scheme-validated, arbitrary query keys
  cannot inject recipients, and launched files reuse the authenticated EML
  import boundary. Failed mailbox actions enter a bounded token-free local
  queue, request Background Sync when available, replay in order on reconnect,
  and use a FYLO-journaled request UUID so lost responses cannot apply twice.
  The complete contract is qualified in Chromium, Firefox, and WebKit.
- Every hosted mailbox maps the stable Inbox, Sent, Drafts, Spam, Archive, and
  Trash roles to safe bounded physical folder paths. Exact-owner category
  discovery, typed FYLO v10 preferences, atomic validation, delivery, list,
  search, move, draft, snooze, reminder, tracking, import/export, and responsive
  settings behavior share the same mapping service. Native defaults preserve
  existing installations, while the contract is reusable by future provider
  connectors without granting cross-domain or same-domain mailbox access.
- The reader presents every authorized RFC conversation message with
  independent expansion, Reply, Reply All, Forward, full-message, and exact
  `.eml` controls. Recognized HTML and plain-text quotations collapse behind
  an accessible disclosure without trusting sender-authored marker attributes.
  Find-in-conversation searches bounded plain-text fields, highlights the
  current result, and supports Enter, Shift+Enter, `mod+g`, and
  `mod+shift+g` traversal.
- Exact GitHub notification mail exposes a reader-toolbar action and the
  upstream `mod+g` shortcut only when the sender is exactly `github.com` and an
  explicit View-on-GitHub anchor resolves to credential-free HTTPS on an exact
  GitHub host. The server derives rather than persists this capability, the
  browser revalidates it, and sender/target lookalikes fail closed without any
  external API or account configuration.
- The participant sidebar can optionally reproduce Mailspring's public GitHub
  contact card. It is off by default, requires an explicit mailbox preference,
  sends only an already-owned contact or message participant to GitHub's fixed
  public API, caches bounded sanitized results, and shows the matched profile
  plus five highest-starred repositories. No token or hosted configuration is
  required, and local contact/history rendering continues when GitHub is
  unavailable.
- The composer has an optional, persisted standalone grammar checker with
  Automatic English, US English, and UK English selection. A bounded local
  LanguageTool-compatible result engine reports grammar, style, punctuation,
  and casing (leaving spelling to the browser); stale and IME-intermediate
  checks are ignored, CSS Custom Highlights annotate without changing draft
  HTML, and an accessible correction list applies replacements or persists an
  ignored rule. No draft text leaves CADUCEUS and no subscription or external
  grammar service is required.
- The rich composer uses semantic selection/range editing for bold, italic,
  underline, strikethrough, inline code, headings, block quotes, code blocks,
  ordered and unordered lists, fonts, sizes, colors, safe links, and RTL
  paragraphs. It supports inline Markdown and automatic URL/email linking,
  To/CC/BCC/From focus shortcuts, roving-toolbar keyboard navigation, and
  accessible pressed state without deprecated `execCommand` behavior. Pasted
  HTML is sanitized before insertion; near-white, near-black, and transparent
  source defaults are removed without discarding independent font styling,
  intentional colors survive, and CRLF/CR plain text becomes stable LF-backed
  editor breaks. Unit and real-browser tests cover the clipboard boundary.
- Reply and forward drafts carry escaped, collapsible, removable quoted text.
  Raster images can be inserted from the picker, paste, or drag/drop, resized
  through accessible controls, and removed atomically with their attachment.
  Draft and send paths retain `inline` disposition and unique Content-ID while
  the live editor uses an inert local preview; outbound HTML contains only the
  corresponding `cid:` reference. A replacement-aware editor/form boundary
  preserves content, recipients, selection controls, and autosave edits across
  Tachyon rerenders in Chrome, Firefox, and WebKit.
- Reader and composer translation use an already-installed browser Translator
  when available and otherwise fall back to CADUCEUS's checksum-pinned,
  self-hosted SMaLL-100 and OPUS models. The authenticated browser reconstructs bounded
  model chunks, verifies every chunk, and runs the matching Asyncify ONNX/WASM
  runtime locally; real inference is qualified in Chrome, Firefox, and WebKit.
  The reader translates sanitized human-readable nodes, retains
  original/translated toggling, and never translates quoted replies; the
  composer preserves signatures, quoted text, code, and template variables.
  Preferences and a 150-message translation cache stay on the device, and no
  message text reaches a hosted service. SMaLL-100 translates the primary 80
  languages directly; two lazy OPUS directions add Maltese, Maori, Mari,
  Basque, Papiamento, Hill Mari, Tajik, Tatar, Telugu, Udmurt, Kyrgyz, Latin,
  and Esperanto through English only when required. The UI therefore exposes
  Mailspring's complete 93-language reader catalogue. The compact checkpoint
  normalizes Hill Mari to its closest Mari constituent, an explicit quality
  boundary that preserves private on-device operation.
- Attachments retain sanitized portable filenames and normalized unique
  Content-IDs across inbound, outbound, draft, and FYLO bucket persistence.
  The authenticated reader resolves only inert raster CID parts into its
  sandbox, provides image/text/PDF previews and exact downloads, and never
  exposes a misleading public link. The composer applies the same 10-file,
  10-MiB-per-file, and 25-MiB-total limits to picker, paste, and drag/drop;
  active SVG content remains download-only.
- Reply-To composer field is validated as one mailbox, carried through
  immediate/delayed/scheduled delivery, emitted in SMTP, and retained on the
  Sent copy.
- Inbound From and Reply-To mailboxes are normalized from MIME headers.
  Messages whose Reply-To differs from the visible From address carry a
  persisted warning, show an accessible warning card, and direct reply actions
  to the declared Reply-To address.
- RFC 2369 and RFC 8058 List-Unsubscribe headers are normalized at delivery,
  unsafe targets are discarded, and localized secure links in non-quoted HTML
  are detected as a final fallback. The reader offers a confirmed one-click
  POST with status/retry feedback, secure preference page, or composer-backed
  mailto action.
- The reader prints an authorized conversation across mailbox folders in
  chronological order, including participants and attachment summaries. Its
  standalone print/PDF document strips executable HTML, remote images,
  tracking metadata, and application state, with unit, route-isolation, and
  real-browser coverage.
- Remote email images and CSS network vectors are blocked before iframe
  rendering by default. Users may load one message, trust an exact sender, or
  opt into ordinary images globally; spam still fails closed. Known provider,
  CADUCEUS, invisible, and tiny tracking pixels are removed even after image
  consent, with typed FYLO preferences and real-browser request assertions.
- Every accepted inbound message retains its exact RFC 822 bytes in a
  tenant-scoped FYLO bucket. The authenticated reader can switch safely among
  protected HTML, plain text, and inert source views, and can download the same
  bytes as an `.eml` file. Sent messages retain a standards-compliant canonical
  MIME source, including Reply-To and attachments.
- Every authorized folder can be exported as a portable mboxrd file from the
  mailbox toolbar. Exact RFC 822 sources stream from bounded immutable FYLO
  pages, the browser assembles byte-safe LF records with standard `From `
  quoting, and live message/byte progress can be canceled without committing a
  partial destination. Multi-domain exports reauthorize each domain separately.
- A concrete mailbox folder can also be exported as a ZIP64 archive containing
  one collision-safe `.eml` file per message. The store-only streaming writer
  preserves each source byte-for-byte, bounds retained metadata, reports live
  progress, and uses the same atomic destination and cancellation guarantees as
  mbox export.
- Screenshot mode can be entered from the account menu or with
  `mod+shift+p`. It masks text, custom-element content, images, CSS
  backgrounds, and same-origin message frames, replaces a potentially
  sensitive window title, survives same-tab navigation, and always exposes an
  explicit exit control. It does not alter or duplicate the underlying
  accessible content.
- Users can import one or many `.eml` files into a concrete authorized mailbox
  and folder. Exact source bytes, MIME attachments, safe rendering metadata,
  unsubscribe signals, dates, and RFC thread identity are retained. The UI
  reports per-file progress, supports cancellation between atomic files, and
  identifies failures without replaying successful imports. Imported files do
  not execute SMTP delivery side effects.
- Inbound and outbound messages persist canonical Message-ID, In-Reply-To, and
  References metadata. Tenant-isolated connected components resolve replies
  across folders without merging unrelated equal-subject mail; the composer,
  quick reply, Sent MIME source, conversation list, and reminder resolver all
  preserve the same RFC lineage. Historical rows retain an explicit subject-
  only compatibility fallback because missing headers cannot be reconstructed.
- Thread rows show two chevrons for a conversation between the signed-in
  mailbox and exactly one other participant, one chevron when additional
  participants are present, and no marker when list delivery does not name the
  mailbox in visible recipient headers. Canonical To/Cc/Bcc addresses are
  retained at inbound and outbound persistence boundaries, and every marker has
  an accessible text equivalent.
- Inbox search provides a cursor-aware token editor with keyboard and pointer
  suggestions for Mailspring's people, category, date, state, attachment,
  subject, and folder terms. A quoted Boolean grammar supports explicit and
  implicit conjunctions, disjunction, negation, and parentheses across both
  message and conversation views. Search remains exact-owner and domain scoped,
  includes attachment filenames, returns complete matching threads, and omits
  Trash and Spam unless the query names a folder explicitly.
- Unread inbox state is reconciled from complete mailbox snapshots into the
  installed-app Badging API using the user's persisted all-hosted-domains or
  focused-domain scope. Unsupported or rejecting platforms receive a live
  `(count)` title fallback that yields to screenshot privacy mode. New-mail Web
  Push payloads use the same preference for an authoritative server-side
  unread count; Tachyon's atomic post-bundle hook extends the root worker with
  badge, notification, deep-link focus, and foreground reconciliation behavior
  without editing generated artifacts.
- Reusable rich composer templates have exact-owner FYLO persistence, bounded
  create/edit/delete APIs, duplicate-safe names, prefix search, draft-to-template
  creation, and a responsive settings editor. Mailspring-compatible
  `<code class="var">` blanks are highlighted and keyboard-navigable; empty
  blanks warn before sending, while all authoring wrappers are removed from the
  delivered message. Template insertion preserves signatures and quoted mail,
  and hostile stored markup is sanitized at server and browser boundaries.
- Per-mailbox rich signatures have exact-owner FYLO persistence, bounded and
  sanitized create/edit/delete APIs, and a responsive settings editor with
  safe rich-clipboard paste and preview. Defaults are assigned to exact owned
  primary or alias addresses; switching the composer From address switches its
  default without duplicates. Manual selection and removal, draft recovery,
  reply/forward placement before quoted mail, alias SMTP delivery, and hostile
  same-domain impersonation rejection are covered by route and browser tests.
- Interface and composer typography can be selected independently from a
  vetted local/system font catalog. Typed FYLO user preferences survive reloads
  and navigation; document-global variables update app chrome, DuVay controls,
  and newly authored message text without remote font requests or arbitrary
  CSS. API, catalog-drift, injection, responsive UI, and browser lifecycle
  coverage keep the boundary observable.
- Four mail lifecycle events can independently select a short local cue or be
  muted. The browser synthesizes the fixed chime, bell, pop, and pulse catalog
  without downloading audio or depending on a hosted service. Typed FYLO
  preferences survive reloads, the settings page provides accessible previews,
  and foreground delivery, send-press, accepted-send, and confirmation paths
  have catalog-drift, injection, failure-tolerance, API, and real-browser tests.
- A local visual gallery selects Automatic, Light, Dark, Darkside, Less Is
  More, Taiga, or Ubuntu profiles. Automatic has independent light and dark
  slots and reacts live to operating-system appearance changes; the topbar
  control remains a one-step Light/Dark override. Typed FYLO v7 preferences,
  fixed token-only profiles, URL/CSS rejection, WCAG contrast assertions,
  reload checks, narrow-screen containment, and every-primary-route Light/Dark
  browser coverage make the complete theme lifecycle observable.
- WCAG 2.2 AA is enforced as a three-engine browser gate across every primary
  authenticated route at phone and desktop sizes, login, and a populated
  reader. Role-driven screen-reader flows, modal focus containment/restoration,
  named loading announcements, keyboard-resizable separators, forced colors,
  reduced motion, WCAG text spacing, and 400%-equivalent reflow are covered;
  `docs/ACCESSIBILITY.md` defines the complementary manual release matrix.
- The reader provides a persistent show/hide Contact control and a responsive
  participant context sidebar. It presents the exact mailbox owner's local
  contact name, organization, title, phone, links, and notes plus a bounded,
  expandable related-conversation history. This standalone adaptation never
  calls Mailspring's hosted enrichment service. Explicit FYLO message ownership
  is authoritative across list, detail, search, threads, attachments, source,
  and live SSE replay; narrow legacy fallbacks fail closed, and a two-user
  same-domain browser test proves another mailbox's history cannot appear.
- Inbox conversations can be snoozed with Mailspring-compatible Later Today,
  Tonight, Tomorrow, This Weekend, Next Week, Next Month, and custom choices.
  One exact-owner FYLO record tracks the RFC thread; a supervised,
  non-overlapping worker returns only messages still in Snoozed, marks them
  unread, completes idempotently, and sends a deep-linked push alert. Due
  labels and a navigation-safe bounded Undo action have real-browser coverage.
- Folder and label pickers now match Mailspring's account-scoped behavior from
  row context actions, bulk actions, and the message reader. Searchable nested
  categories can be created in place; labels expose none/mixed/all state and
  use apply-all/remove-all semantics. Mutations are exact-owner, durable,
  undoable/redoable, keyboard accessible, responsive, and covered in Chromium,
  Firefox, and WebKit.
- Application notices now follow Mailspring's derived priority stack: offline,
  locally installed update, mailbox reconnect, disabled-rule, and optional
  notification-enablement states. The highest-priority item remains visible in
  the sidebar, while a responsive topbar panel exposes every active notice and
  its bounded action. Required health notices cannot be dismissed; optional
  dismissals are user-scoped and durable on the device. Existing typed FYLO
  preferences continue to govern notification accounts, privacy, actions,
  badges, and local sound cues.
- Composer state is durably autosaved to FYLO with an idempotent client key,
  optimistic versions, complete attachment replacement, exact mailbox-owner
  isolation, AES-GCM encrypted IndexedDB crash recovery, and explicit cross-tab conflict choices.
  Saved drafts reopen from the Drafts mailbox and are consumed after sending or
  permanently removed with their attachment bytes when discarded.
- One session-bound encrypted IndexedDB vault now serializes offline mailbox
  actions, draft autosaves, Send and Send Later, snooze, and calendar writes.
  Reconnect and Background Sync share one per-mailbox replay pass, never persist
  bearer tokens or plaintext message bodies, and use durable server-side replay
  identities so a lost response cannot duplicate a send, snooze, or calendar
  create. The same vault now carries the bounded mailbox body/attachment index
  and its offline structured-search projection.
- Follow-up reminders use an accessible composer chooser, persist against the
  canonical Sent row and RFC thread identity, resolve only for a matching
  participant reply, and can be listed, opened, or dismissed by their exact
  mailbox owner. A signed supervised worker processes every mailbox without an
  inbox read, prevents overlapping passes, reconciles deterministic notices
  after a crash, and emits bounded success/failure/result measurements. Unit,
  route, runtime, and real-browser tests cover the complete lifecycle.
- Mailbox-isolated contacts support personal records, automatic found-in-mail
  learning, groups that can be removed without deleting their contacts,
  responsive list/detail editing, accessible composer completion, and tested
  vCard import/export. Optional CardDAV accounts now have authenticated
  principal/home/book discovery, bounded vCard ingestion, exact
  tenant/principal/account FYLO isolation, atomic snapshot visibility, RFC 6578
  incremental sync-token deltas, conditional ETag writes, durable mutation and
  conflict journals, ordinary contact search/completion, and protocol fixtures.
  The authenticated editing API lists redacted owner-scoped address books and
  queues create/update/delete writes with idempotent status polling. The server
  generates remote identities, uses create-only and ETag preconditions, and
  reconciles ambiguous responses without duplicating a contact. Stale-ETag
  conflicts refresh the server version and offer explicit review/edit or keep
  choices. RFC vCard 3/4, URI-valued, and grouped Apple-style properties have
  parser and non-destructive merge fixtures. The contacts package is
  `implemented`. Gmail/Google Workspace accounts additionally synchronize and
  write through Google People with persisted sync-token fallback; Microsoft
  accounts discover and synchronize Microsoft Graph contact folders and use
  delta links where supported. Both native adapters share the exact-owner FYLO
  projection, enforce provider writability and source ETags, and reconcile
  retried creates through provider-stored opaque idempotency markers.
- Outbound open and link tracking is explicit opt-in per message and recipient.
  The server issues encrypted, expiring capabilities, rewrites only non-quoted
  links, rejects redirects not recorded for that message, and keeps the Sent
  HTML and canonical MIME source free of tracking artifacts. The mailbox-owned
  Activity page provides unseen event counts, open/click filters, bounded
  daily and subject analytics, and per-recipient event details. Composer and
  reader disclosures call out proxy and prefetch limitations; route, unit, and
  real-browser tests cover defaults, delivery transformation, owner isolation,
  event ingestion, dashboard navigation, and detail presentation. Connected
  accounts use the same clean-Sent/provider-delivery split for one or many
  envelope recipients. Recipient-specific MIME buckets are staged before one
  atomic journal commit; an interrupted edit preserves the former runnable
  batch, and any partial SMTP delivery becomes terminally ambiguous rather than
  being replayed.
- Send-capable external IMAP/SMTP accounts now appear as composer identities.
  Their raw RFC 822 source is journaled in a bounded FYLO bucket before the Go
  worker uses server-owned encrypted credentials under the account lease.
  Idempotent enqueue, undo, scheduled cancellation, recipient refusal, retry,
  and post-DATA ambiguity are covered by Go adapter tests and the authenticated
  runtime route suite. Connected-account Send & Archive and its Undo Send
  restoration are covered by the real IMAP/SMTP fixture. Revision-checked
  in-place schedule editing retains the canonical ID while replacing
  recipients, content, attachments, reminders, delivery time, and the bounded
  FYLO MIME buckets. Connected tracking for both single and grouped recipients,
  staged batch replacement, Undo Send, and scheduled cancellation are covered
  by the real SMTP fixture. IMAP capability negotiation enables QRESYNC for
  bounded flag/expunge deltas with an atomic durable modification sequence.
  CONDSTORE-only servers use restart-safe UID-range checkpoints plus explicit
  presence windows, while servers without either extension retain the existing
  bounded UID reconciliation behavior. A separately supervised, bounded IDLE
  watcher turns Inbox `EXISTS`, `RECENT`, `EXPUNGE`, and `FETCH` notifications
  into durable scheduler wakes; only the lease-owning sync worker writes FYLO,
  and unsupported servers retain the minute poll fallback.
- Each mailbox owner can explicitly enable a stateless Model Context Protocol
  endpoint, choose exact accounts, exclude physical folders, and grant read,
  write, or send authority. A dedicated one-time capability is hash-only in
  FYLO, rotates and revokes independently of SESAME sessions, and authenticates
  its tenant before lookup. The official MCP SDK exposes the complete upstream
  account, folder, search, thread, message, tracking, draft, mutation,
  scheduled-send, and delivery tool surface; exact-owner and exclusion checks
  apply inside every tool and batch. Settings also expose the endpoint and a
  bounded durable activity view that records parameter names but never mail or
  token values.

## Updating the baseline

When Mailspring advances, inspect the new upstream commit and changelog before
changing `upstream.commit`. Add every new package or capability to the ledger;
never silently move the baseline forward while omitting a new requirement.
The audit script intentionally pins the complete sorted package inventory so a
missing package is a structural failure.
