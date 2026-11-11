# ADR 0006: Offline-first clients: a device copy, a sync log per conversation, an offline queue

- Status: Accepted (design; build after Wave 3, one login)
- Date: 2026-11-11
- Builds on: [ADR 0004](0004-mesail-go-serverless-keycloak.md) and
  [ADR 0005](0005-one-login-per-person.md). Nothing in them is replaced.
- Spec: [SPEC.md](../mesail/SPEC.md) S15 to S17, AC-OF-1 to AC-OF-17,
  Q8 to Q14. Contracts: [CONTRACTS.md](../mesail/CONTRACTS.md)
  "messaging" (sync), "identity" (kept devices). Slices:
  [SLICES.md](../mesail/SLICES.md) "Wave 4: offline first".
- Amended: 2026-11-11, by TACHYON's approved plan for T1 to T12 (section
  9). Where section 9 and an earlier section differ, section 9 wins.

## Context

The owner's requirement (2026-11-11): "i was thinking we could also do
Local-First for this application". The owner's answers:

1. **Offline-first**, not local-first: the server stays the source of
   truth. Each device keeps a copy of recent conversations and mail, works
   offline (read, search, write), queues sends and edits, and syncs when it
   is back online.
2. **Recent, then on demand**: the last days or messages per conversation
   and mailbox; older items are fetched when opened, so phones stay light.
3. Design now, build after the one-login wave.

What the platform gives today (research of 2026-11-11, at the release tags):

- **TACHYON 26.46.02, Go Tac.**
  - Go runs as one WebAssembly module on the page's main thread
    (`GOOS=js GOARCH=wasm`, TinyGo for release builds). It may not use
    `syscall/js` or `net/http` (TY1317). It has no file system, no threads
    and no workers.
  - The only persistent structured storage for Go is
    `@Entity({"adapter":"indexeddb"})` with a `Store` repository
    (`findBy`, `@Query`, paging, `@AllOrNothing`), in the IndexedDB
    database `tachyon-store` (TACHYON ADR 0059). Before a Go service
    starts, every table is copied into memory. A table over 10,000 rows is
    not copied, and a Go service that uses it stops. Writes go to IndexedDB
    in the background, in order.
  - `@LocalState` and `@SessionState` are synchronous JSON fields of at
    most 64 KiB each. `@Cached` uses the Cache API, also fully loaded into
    memory.
  - There is no blob store, no OPFS, no `navigator.storage.persist()`, no
    WebCrypto for services (so no non-extractable keys), no key store and
    no sync between tabs (each tab has its own in-memory copy).
  - `ConnectivityChanged{online}` exists. `@Retry` (in memory) exists. A
    persistent request queue does not.
  - The service worker precaches the app's files, so a web app opens
    offline. It never caches a request with an `Authorization` header.
  - Sockets reconnect after 1, 2, 4, 8 and 16 seconds, then stop.
  - Native apps (macOS and iOS WKWebView, Windows WebView2, Linux
    WebKitGTK, Android WebView) are WebView shells around the same bundle.
    They have the same web storage, which survives a relaunch, and no
    native storage or key store API. The Apple scheme `tachyon-app://` is
    not a secure context.
- **FYLO 26.45.07 on a device.**
  - `fylo-browser.wasm` keeps a `.vdbx` file in OPFS from a Worker. It is
    tested in Chromium and Firefox, not in WebKit, and has no OPFS in
    WKWebView. There is no iOS or Android build and no C ABI.
  - Go in the browser cannot call it: the Go client starts a process.
  - It has no full-text search, no ORDER BY, no change feed, no sync, and
    no encryption in the browser engine without cross-origin isolation.

## Decision

### 1. The device copy: TACHYON's IndexedDB `Store`, behind a Mesail port
- The web app keeps the device copy in TACHYON's `Store` with the
  IndexedDB adapter, the one persistent store that Go Tac has on every
  platform: browsers, and the WebViews of every native app. FYLO is not
  used on the device: it cannot run in WKWebView, Go cannot call it, and it
  lacks search and ordering.
- All client code reaches the copy through one port, `DeviceStore`
  (`apps/web/client` package `devicestore`): partitions, records, the
  queue, cursors, wiping and the space budget. Only its adapter knows
  TACHYON's `Store`. A later adapter (native SQLite, or FYLO when it runs
  in WebKit and on phones) replaces only the adapter.
- **Partitions.** The copy is split by person and domain: partition
  `<personId>/<domainId>`. Each partition has its own conversations,
  messages, cursors, queue and drafts. This is how the one-login domain
  switch fits (ADR 0005): switching domains opens another partition, also
  offline; a queued item is sent only with a token for its own domain.
- **Records are pages, not single messages** (because of the 10,000-row
  limit): one row per conversation and per day holds that day's messages
  of the conversation as server-made `Message` values. A 30-day window of
  300 active conversations stays under 9,000 rows.
- **Until TACHYON copies tables lazily** (request T1), every row is held
  in the WebAssembly memory of the tab. So the first build keeps a small
  window: 7 days, at most 5,000 messages and 40 MiB per person, no
  attachments offline. The full window of AC-OF-2 (30 days on a computer,
  14 on a phone, 1 GiB) switches on with T1, T2 and T9, without a change of
  the sync protocol.

### 2. The kept device and the long session
- A device copy exists only on a **kept device**: at sign-in the person
  ticks "Keep me signed in on this device". It is on by default in the
  installed apps and off in a browser (SPEC Q8). Without it, nothing stays
  on the device after the tab closes, as today.
- On a kept device `identity` asks Keycloak for an **offline session**
  (scope `offline_access`; idle 30 days, at most 90 days). The
  `sessionKey` seals that offline refresh token and is stored in the
  device copy (encrypted, section 6) instead of in `@SessionState`, so the
  app opens offline after a restart. The Mesail access token still lives
  60 seconds and only in memory.
- **Ending access on other devices.** The person record in `directory`
  gets `sessionsEndedAt`. It is set when the person signs out everywhere,
  changes their password (all other sessions), is reset or disabled by an
  admin (AC-ID-13). A refresh whose session started before it answers
  `403 access_revoked`, and that device deletes every copy of the person.
  A plain `401` (the session expired) keeps the copy locked until the same
  person signs in again.

### 3. Sync: a change log per conversation, the realtime socket as a poke
FYLO has no change feed and no ORDER BY, so `messaging` keeps its own log.
- **The change log.** `messaging` appends one row per change of a message
  to `changes-<domainKey>-<YYYYMM>`: a message created, edited, deleted,
  a reaction changed, a reply count changed. A row holds ids only:
  `changeId` (the FYLO TTID, time-ordered), `conversationId` (indexed),
  `messageId`, `eventId` (indexed, for idempotency). The row is written by
  `messaging`'s outbox relay when it relays the event (drain on write,
  EVENTS.md section 2), before the event is pushed. `messaging` is the
  only writer of its domain's root (ADR 0004 section 4), so rows are
  appended in order. Rows are kept at least 30 days: the current and the
  two previous months.
- **The conversation list** (`GET /messaging/conversations`) gains
  `lastChangeId` and the caller's `readMessageId` for each conversation.
  A conversation whose `lastChangeId` differs from the device's cursor has
  changes.
- **The sync route** `POST /api/v1/messaging/sync` takes up to 50
  `{conversationId, after}` cursors and answers, per conversation, the
  messages that changed after the cursor, each as its **current full
  `Message`** (the server's state wins; no deltas), up to 200 per page,
  with the next cursor. It reads from 5 seconds before the cursor and the
  device applies messages by id, so a row committed late is never missed
  and a repeated row changes nothing. A cursor older than the log answers
  `reset` (the device reloads the conversation's window); a conversation
  the caller can no longer read answers `gone` (the device deletes it).
- **Catch-up** after being offline: (1) refresh the token; (2) read the
  conversation list (all pages) and the channel list; (3) delete the
  conversations that are gone; (4) sync, 50 conversations per call, every
  conversation whose `lastChangeId` changed, newest first; (5) send the
  queue (section 4). New conversations are loaded with their window.
- **Realtime is only a poke.** On `message.*` and `reaction.changed`
  events the device syncs that one conversation (at most once per second).
  On `read.moved` it applies the read position. A reconnect, the end of
  the socket's five retries or `ConnectivityChanged{online: true}` starts
  a catch-up. So the socket never has to deliver every event: the sync
  route is the only data path.
- Email conversations are conversations: mail syncs the same way. There
  is no separate mailbox protocol.

### 4. The offline queue
- Every change made on the device is first written to the partition's
  queue, then shown, then sent: send (with the existing `clientMessageId`),
  edit (with `ifVersion`), delete, reaction add and remove, read position.
- Items of one conversation are sent in order; conversations in parallel
  (at most 4 requests at a time). Network errors, `429`, `503` and other
  `5xx` are retried with backoff from 2 seconds to 5 minutes while the
  device is online. `401` refreshes once. Every other refusal moves the
  item to **Needs your attention** with the reason (AC-OF-6); items that
  depend on it (an edit of an unsent message) go with it.
- The server makes every queued operation safe to repeat: a send by
  `clientMessageId` (AC-MS-8), an edit by `ifVersion`, a delete of a
  deleted message answers `204`, reactions are a set, read positions only
  move forward. So two tabs that both send the same item cause no
  duplicate (TACHYON has no sync between tabs, request T11).
- **Conflicts** (AC-OF-8): an edit carries `ifVersion` (the message's
  `version` the device edited); a newer version answers `409
  edit_conflict` with the current message, and the device keeps the
  offline text as a draft. Any edit or reaction on a deleted message
  answers `409 message_deleted` and is dropped. Messages are ordered by
  the server's `createdAt`; `writtenAt` (the device time) is shown only.
- **Lost access** (AC-OF-9): `403 not_a_member` or `gone` deletes the
  domain's partition or the conversation after the person sees how many
  queued items were not sent; `403 access_revoked` deletes every partition
  of the person.
- Limits: at most 500 items per partition.

### 5. Search offline
- Offline, the web app searches the device copy with the server's query
  rules (word prefixes of body, subject, sender and file name; AC-SR),
  from a word index it builds in memory from the copy when the device
  goes offline or the copy changes. Online, it uses the server's search.
  Results from the device are marked. No search index is stored.

### 6. Size, eviction and wiping
- The budget (AC-OF-13) counts rows and bytes per person. The adapter
  removes, in order: on-demand pages, then the oldest pages outside the
  window. It never removes queued items or drafts.
- Wiping (sign-out, another person, `access_revoked`, a lost domain,
  "Remove the copy on this device") deletes the partition's rows in one
  `@AllOrNothing` call, then the stored `sessionKey`.
- Without `navigator.storage.persist()` (request T9) a browser may delete
  IndexedDB under storage pressure, and Safari deletes a web site's
  storage after 7 days without a visit (not for an installed web app). The
  copy is only a copy, so this loses nothing but queued items; the
  interface says when items are waiting, and the installed apps are the
  recommended way to work offline.

### 7. Data at rest
- The device copy holds no password and no Keycloak token. The stored
  `sessionKey` is already sealed by `identity`.
- **Today there is no safe place for a key on the device**: no WebCrypto
  for services, no non-extractable keys, no key store, and the Apple
  scheme is not a secure context. A key stored next to the data protects
  nothing. So the first build relies on the operating system's disk
  encryption, on the kept-device choice (off by default in a browser), on
  wiping, and on locking the copy after 30 days without a successful sync
  (SPEC Q10). The record format keeps a `keyId` field, so that when
  TACHYON offers a key store (requests T6 and T7) the rows are encrypted
  with AES-GCM under a device key held by Keychain, Android Keystore,
  Windows DPAPI or the Linux secret service, without another change.
- AC-OF-14 holds fully only after T6 and T7. Until then the release notes
  say so, and the browser default stays off.

### 8. What does not change
- The server is the source of truth. No data is merged on the device:
  the server's `Message` always replaces the device's.
- No new event types. Chat still never crosses a domain; a partition is
  one domain.
- `web-admin` keeps no device copy: admin work needs the network.

### 9. Amendment: TACHYON's approved plan (2026-11-11)
The owner approved TACHYON's plan for T1 to T12 (the TACHYON section's
`tachyon-offline-first-plan.md`). It changes this ADR as follows.

- **Store (T1, TACHYON wave 1, ADR 0123).** A `Store` table may declare a
  partition field (`@Entity({"adapter":"indexeddb","partition":"..."})`);
  only eager tables are copied at start, a partition is loaded and
  unloaded on demand, and the 10,000-row limit applies per partition. So
  the `DeviceStore` adapter uses **one row per message**, partitioned by
  `partitionKey` = `<personId>/<domainId>/<conversationId>`; the
  conversation list, cursors, queue and drafts are small eager tables. The
  device opens a conversation by loading its partition and unloads
  partitions to stay in memory budget. The "one row per conversation and
  day" layout of section 1 is used only if slice O3 must ship before T1 is
  in a TACHYON release. A failed write in IndexedDB (for example a full
  disk) now arrives as `StoreWriteFailed`; Mesail then stops adding to the
  copy, keeps the queue, and says "This device is full" (SPEC AC-OF-13).
- **Space (T9, wave 1, ADR 0124).** On a kept device the app calls
  `StorageSpace.persist()` once (Firefox shows its own prompt) and uses
  `estimate()` for the space settings and the budget.
- **Sockets (T5, wave 1, ADR 0125).** The app sets a reconnect policy that
  never gives up (backoff up to 60 seconds), and names the realtime
  gateway as a second origin if it is not on the API origin.
- **Push (T4).** Web Push for the browser already exists in TACHYON (ADR
  0118, after 26.46.02); T4a (wave 1) adds `PushReceived` for open pages,
  which the app treats as one more sign to sync. Native push (T4b) comes
  later. Sending Web Push from Yon (T4c) is not built and not needed:
  Mesail's `notify` sends with its own Go `kit/webpush`. (Using browser
  push in Mesail changes SPEC AC-PN-3; that is a separate decision.)
- **Blobs (T2, wave 2, ADR 0126).** Attachments in the copy use the
  `Blobs` client (IndexedDB, quota from the manifest). TACHYON never
  deletes a blob itself; Mesail's budget code does (section 6).
- **Tabs (T11, wave 2, ADR 0127).** Store changes reach the other tabs
  (`StoreChanged`), and a `Locks` client lets one tab send the queue. The
  native apps have one window each; several windows of one native app are
  not supported.
- **Encryption at rest (T6 + T7, wave 3, ADR 0128): sealed tables.** An
  entity declares `"sealed": true` (and the blob store
  `tac.storage.blobs.sealed`): each row is stored as AES-GCM ciphertext
  under a device key that service code never sees. The key is a
  non-extractable WebCrypto key on the web; the OS key store in the native
  apps (Keychain with CryptoKit on macOS and iOS, Android Keystore, Windows
  DPAPI, the Linux Secret Service). `StorageSeal.forget()` makes every
  sealed row unreadable at once, which Mesail calls first on every wipe
  (section 6). Fields used by an index stay plain: the partition key
  (person id, domain id, conversation id) and the message id are readable
  on disk; bodies, names, addresses, subjects and attachments are not.
  On Linux without a running Secret Service, and on a plain-`http` web
  origin, a sealed table refuses to open (see SPEC Q15). Mesail declares
  every device-copy table and the blob store sealed, so section 7's
  `keyId` field is no longer Mesail's: the storage layer keeps it.
- **Outbox (T3): not built.** Mesail keeps its own queue (section 4),
  sends it from the tab that holds the T11 lock, and sends only while the
  app is open.
- **Workers (T10, later):** worker services have no view. The sync engine
  and the offline search are written as services with no view binding, so
  they can move into a worker when T10 lands.
- **Not coming:** native storage (T8, deferred until a 1 GiB relaunch test
  fails; web storage already survives a relaunch in all shells) and a
  larger field limit (T12; T1 and T2 cover it).

So the full window of AC-OF-2 switches on with T1, T9 and T2 (TACHYON
waves 1 and 2), and AC-OF-14 holds fully with sealed tables (wave 3).

## Options considered

- **FYLO on the device** (`fylo-browser.wasm` in OPFS). Rejected for now:
  no WKWebView (iOS and macOS), no phone builds, Go cannot call it, no
  full-text search or ordering. It may replace the `DeviceStore` adapter
  later (requests F1 to F4 in the dependency list).
- **A JavaScript service that talks to IndexedDB directly**, sending
  events to Go. Rejected: TACHYON does not support it as a path, and it
  splits the client's logic between two languages.
- **One change feed per person** (fan-out of every change to every
  reader). Rejected: a channel of 500 members would cost 500 writes per
  message, against the 25-writes-per-call rule (SLICES rule 9). A log per
  conversation costs one row per change and one list read per catch-up.
- **Deltas instead of full messages in the sync answer.** Rejected: full
  messages make every apply idempotent and need no merge rules.
- **Last edit wins.** Rejected as the default (SPEC Q12): it can silently
  undo the author's newer edit from another device.
- **Local-first** (the device as an equal source, merged with CRDTs).
  Rejected by the owner (answer 1): the server stays the source of truth,
  email has to pass the server anyway, and access rules are server rules.

## Consequences

Good:
- The protocol needs only one log collection, three new fields and one
  route on the server, and works the same for chat and mail.
- Every queued operation is safe to repeat, so retries, two tabs and lost
  answers cause no duplicates.
- The `DeviceStore` port lets the store improve (T1, native stores, FYLO)
  without touching sync, queue or interface code.

Costs, accepted:
- Until TACHYON ships T1, T2 and T9 the window is small (7 days, 5,000
  messages, no attachments offline), and the copy lives in each tab's
  memory.
- Until T6 and T7, data at rest is protected only by the operating
  system, the kept-device choice, wiping and the 30-day lock.
- Catch-up reads the whole conversation list each time (one call per 100
  conversations).
- The change log adds one write per relayed message event in `messaging`.

## Requests to dependencies

Status after TACHYON's approved plan (section 9): T1, T5, T9 and T4a in
wave 1; T2 and T11 in wave 2; T6 and T7 as sealed tables in wave 3; T10 as
worker services without a view, later; T4b later; T3, T4c, T8 and T12 not
built. The original requests:

TACHYON (Go Tac), in order of need:
- **T1** `Store` tables for Go services read lazily and by page from
  IndexedDB, without copying the whole table into memory, and without the
  10,000-row limit.
- **T2** A blob store for services (binary values, megabytes each:
  attachments), with a quota.
- **T3** A persistent outbox for `HTTPClient` calls, replayed when online,
  with Background Sync where the browser has it. (Mesail builds its own
  queue in the `Store` meanwhile; T3 would let a queue send while the app
  is closed.)
- **T4** Web Push (PushManager), and APNs and FCM in the native shells
  (already requested in ADR 0004).
- **T5** Sockets and event streams with a reconnect policy set by the app
  (no stop after five tries), and to a second configured origin (the
  realtime gateway).
- **T6** WebCrypto for services, with non-extractable AES-GCM keys stored
  in IndexedDB, and a secure context on the Apple scheme
  `tachyon-app://`.
- **T7** A key store in the native shells: Keychain (macOS, iOS), Android
  Keystore, Windows DPAPI or CNG, Linux secret service.
- **T8** Native storage for the shells: an app data folder, or a native
  SQLite adapter for `Store`.
- **T9** `navigator.storage.persist()` and `estimate()` for services.
- **T10** Workers for WebAssembly services, so sync and search run off the
  main thread.
- **T11** Sync between tabs: changes to `Store` and `@LocalState` seen by
  the other tabs of the app, and a lock (Web Locks) so one tab sends the
  queue.
- **T12** A larger field limit than 64 KiB for socket messages and state
  (or the blob store of T2).

FYLO (only if FYLO is to become the device store later):
- **F1** WebKit and Safari support, with OPFS or an IndexedDB backend
  that works in WKWebView, and recovery and soak tests passing.
- **F2** iOS and Android builds with a C ABI (an `.xcframework` and an
  AAR), and an iOS Keychain and Android Keystore identity store.
- **F3** Full-text search (an inverted word index) and ordered, limited
  queries on an indexed field.
- **F4** A change feed (subscribe to the changes of a collection) and
  encryption in the browser engine without cross-origin isolation.

DuVay:
- **D1** Ready-made patterns for the offline states: an "Offline, updated
  at" status bar, a "Waiting to send" message state, a "Needs your
  attention" list, and a space-used meter. Mesail builds them with
  Tailwind meanwhile (ADR 0004 section 8).
