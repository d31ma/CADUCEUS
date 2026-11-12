# Mesail events and the outbox

Slice I2. This page tells a service slice how to publish and consume events.
The code is in `kit/events` and `kit/outbox`. Payload fields are in the Go
types in `kit/events` (one file per event type). The table of producers and
consumers is `kit/events` `Catalog` (the same table as DOMAIN.md section 4).

## 1. Rules

- **Every event carries the domain.** `Envelope.domainId` is the mail
  domain (the tenant). Payloads do not repeat it. A consumer writes only into
  `envelope.domainId`. Chat events never move data into another domain.
- **At least once, applied once.** The relay can push an event more than
  once (for example after a crash between the push and the save). The
  consumer drops a repeat by the key (`domainId`, `eventId`).
- **Only the listed producer.** A consumer refuses an event whose `producer`
  is not the `Catalog` producer of its type, and an event type its service
  does not consume.
- **One FYLO write.** The event is staged inside the document the change
  writes (FYLO has no transactions across collections), so the change and
  its event are durable together or not at all.
- **One writer.** Each service that owns a FYLO root runs as a Lambda with
  reserved concurrency 1 (ADR 0004), so the relay and the routes never write
  the same document at the same time. The relay also merges its result onto
  a fresh read of the document before it saves, so an event staged during a
  relay run is kept.

## 2. What a producing service adds

### 2.1 Document fields

Every FYLO document that can carry events has six more fields (FYLO
rejects arrays of objects, so each pending event is one JSON string):

| Field | Type | Meaning |
| --- | --- | --- |
| `pendingEvents` | string[] | `outbox.PendingEvent` as JSON: the envelope, the subscribers still waiting, attempts, next attempt time, last error |
| `deadEvents` | string[] | `outbox.DeadLetter` as JSON: one per event and subscriber that gave up |
| `outboxPending` | int | number of pending events |
| `outboxDead` | int | number of dead letters |
| `outboxWaiting` | bool | `true` while `outboxPending > 0`; the sweep finds documents with the exact match `outboxWaiting: true` |
| `outboxHasDead` | bool | `true` while `outboxDead > 0`; the dead-letter view finds documents with `outboxHasDead: true` |

The two booleans are set in the SAME patch as the counts, every time
(`kit/outbox` computes all six fields together). They exist because
Tachyon's `Scroll` filters only by exact match (`$eq`, at most 8 fields),
so `outboxPending > 0` cannot be written, and a sweep that reads every
document and filters in memory grows with all stored data (found by slice
M1, whose gate timed out on a grown test root). `$eq` on a field is also
what FYLO indexes.

**The sweep rule for every producer** (D1, C2, M1, M2, M3, N1, R1, S1 when
they produce): scroll each registered collection with
`{"outboxWaiting": true}` only, stop after the SLICES rule 9 limit (25
writes or 20 seconds), answer `more: true` when it stopped early, and
never scroll a collection without that filter. Tests remove the documents
and collections they create, so a test root does not grow across runs.

### 2.2 A newer event replaces a waiting one (decided 2026-11-12)

A document must not grow with every change while a subscriber is down
(found by K1: each edit staged a `message.edited` with the full body, so
many edits during an outage grew one message without limit). So:

- An event type may declare a **replace key** in `kit/events` (for
  example `message.edited` and `reaction.changed`: the message id;
  `presence.changed`: the member id). When `kit/outbox` stages an event
  whose type and replace key match an event still waiting on the same
  document, the new event replaces the waiting one (its subscribers are the
  union of both lists), instead of being added.
- `message.deleted` also removes any waiting `message.edited` of that
  message.
- The same applies to `deadEvents`: a new dead letter of the same type and
  replace key replaces the older one for that subscriber.
- This is safe because these events carry the current state (EVENTS.md
  section 7) and every consumer applies them idempotently; a consumer that
  missed an older edit only ever needs the newest one.
- Events without a replace key (`message.created`, `email.*`,
  `channel.*`, `member.upserted`, ...) are added as now; each is staged
  once per change of its own document, so they do not pile up.
- Every waiting event still counts against the document's 512 KiB cap
  (DOMAIN.md); `kit/outbox` refuses to stage an event that would pass it,
  and the write answers `503` so the caller retries later.

The entity model in `server/models/` declares these six fields with these
JSON names. The store adapter converts them to and from `outbox.Fields`.

### 2.2 Staging an event in the write

```go
envelope, err := events.NewEnvelopeFactory("channels", time.Now, rand.Reader).
	New(events.Types{}.ChannelCreated(), caller.DomainId, events.ChannelCreated{...})
if err != nil {
	return Channel{}, err
}
fields, err := outbox.NewOutbox(settings).Stage(channel.outboxFields(), envelope)
```

Put `fields` back on the document, then write it with one `Insert` or
`Patch`. After the write, call `Relay.RelayDocument(ref)` so the event goes
out in the same invocation (low latency in Lambda form). If that push fails,
the scheduled sweep retries it.

### 2.3 Settings in `server/application.json`

```json
{
  "outbox": {
    "producer": "channels",
    "subscribers": [
      { "name": "messaging", "url": "http://127.0.0.1:8105/api/v1/messaging/events" },
      { "name": "realtime", "url": "http://127.0.0.1:8108/api/v1/realtime/events" },
      { "name": "search", "url": "http://127.0.0.1:8109/api/v1/search/events" }
    ],
    "retry": { "basedelayseconds": 5, "maxdelayseconds": 900, "maxattempts": 12 }
  },
  "outgoing": { "hosts": ["127.0.0.1"] }
}
```

- A subscriber gets an event only when `Catalog.ConsumersOf(type)` names it
  and it is listed here. A consumer that is not listed is not wired in this
  deployment and is skipped (the event is not kept for it).
- Every subscriber host must also be in `outgoing.hosts` (Tachyon's egress
  guard refuses other hosts).
- Read the settings with `outbox.SettingsReader{}.FromEnvironment()`. It
  reads `YON_APPLICATION_JSON` and takes `outbox` from under `yon` (a
  top-level `outbox` key still works, for tests).
- Keys in `application.json` must be lowercase (TACHYON refuses others with
  TY2010); Go's JSON decoding still fills the camel-case struct fields. In a
  handler the settings arrive in `YON_APPLICATION_JSON` under the top-level
  key `yon` (`{"yon":{"outbox":{...}}}`), so readers look there, as
  `kit/oidc`'s `SettingsReader` does.
- Retry: the delay is `basedelayseconds * 2^(attempts-1)`, at most
  `maxdelayseconds`.
  After `maxAttempts` the remaining subscribers become dead letters. A `4xx`
  answer (except `408` and `429`) is permanent and dead-letters at once.

### 2.4 Role files

| File | What it does |
| --- | --- |
| `server/services/OutboxStoreService.go` (`@Service`, `@Needs dataSource DataSource`) | implements `outbox.Store`: `Pending(limit)` scrolls the service's collections with the exact match `outboxWaiting: true`, at most `limit` documents; `Load(ref)` reads one document's six fields; `Save(ref, fields)` patches exactly those six fields |
| `server/services/OutboxRelayService.go` (`@Service`, `@Needs` the store service) | builds `outbox.NewRelay(store, outbox.NewHTTPPusher(tokens, timeout), settings, time.Now)` and exposes `RunOnce` and `RelayDocument` |
| `server/controllers/OutboxController.go` (`@RequestRoute("/api/v1/<service>/outbox")`, `@Allow(Service)`) | `POST /relay` runs `RunOnce` and returns `outbox.Report`; `GET /dead` lists dead letters; `POST /dead/requeue` moves one back (`DeadLetters.Requeue`) |
| `server/workers/outbox-relay.go` (`@Schedule({"fixedDelaySeconds":60})`) | the sweep while the service runs as a long-lived `ty start` process (see section 4) |

`Pending` must know the collections to scan: they are split by domain
(`kit/collections` `CollectionName{}.ForDomain(base, domainId)`, for
example `channels-a379a6f6eeafb9a5`). Keep them in a small install-wide collection,
`outbox-collections`, and add a collection name to it before the first
document is written into that collection. Collection names come only from
the service's own code, never from a request.

**The sync log (ADR 0006).** `messaging`'s relay also appends one row to
its `changes-<domainKey>-<YYYYMM>` collection for every `message.created`,
`message.edited`, `message.deleted` and `reaction.changed` it relays,
before it pushes the event, keyed by `eventId` so a repeated relay adds no
second row (DOMAIN.md "Change"). Devices sync from this log, not from the
events; realtime events are only a sign to sync. No event type changes.

The service token for pushes comes from `identity`'s service token route
(ADR 0005; token role `mesail-service`) through `outbox.TokenSource`.

## 3. What a consuming service adds

`POST /api/v1/<service>/events`, `@Allow(Service)`, body
`events.Envelope`, answer `200 EventReceipt{eventId, status}` where
`status` is `applied` or `duplicate`.

| File | What it does |
| --- | --- |
| `server/controllers/EventsController.go` (`@RequestRoute("/api/v1/<service>/events")`, `@Allow(Service)`) | `PostEvent(@FromBody envelope)`: calls the events service with the caller and the envelope |
| `server/services/EventsService.go` (`@Service`) | `events.NewReceiver("<service>", seen, handler).Receive(caller, envelope)`; the handler switches on `envelope.Type` and reads the payload with `events.PayloadReader{}.Read` |
| `server/services/SeenEventsService.go` (`@Service`, `@Needs dataSource DataSource`) | implements `events.SeenStore` with the collection `CollectionName{}.ForDomain("events-seen", domainId)` (one document per `eventId`, looked up with `$eq`) |

`Receiver.Receive` checks the `Service` role again, checks the envelope
(event id, domain name, known type, the listed producer, time, payload),
checks that this service consumes the type, drops a repeat, calls the
handler, and remembers the event only after the handler succeeded. A handler
error makes the relay retry later.

Answers the relay understands:

| Answer | Relay does |
| --- | --- |
| any `2xx` | delivered |
| `400`, `401`, `403`, `404`, `409`, `422` (any `4xx` except `408`, `429`) | dead letter now (a bad event will not get better) |
| `408`, `429`, `5xx`, network error, timeout | retry with backoff |

So the error handler maps `InvalidEnvelope` and `NotSubscribed` to `400`,
`NotService` to `403`, and storage errors to `503`.

## 4. Running the relay (and any other scheduled work)

Decided by architecture on 2026-11-10, after slice M2 measured that in
TACHYON 26.46.02 a `// @Schedule` worker is compiled alone and gets no
injected providers.

**The standard pattern for all scheduled work** (the outbox relay, the
outbound delivery queue, retries, email summaries, cleanup):

1. The work lives in an ordinary controller route of the service, so it has
   its injected services and stores:
   - the outbox relay: `POST /api/v1/<service>/outbox/relay`;
   - any other job: `POST /api/v1/<service>/jobs/<job>` (for example
     `/api/v1/mail-out/jobs/deliver`).
   The route is `@Allow(Service)`, takes no body (or a small typed one),
   does a bounded amount of work per call, is safe to call twice at the same
   time, and answers a typed `Report` of what it did.
2. Something only triggers that route:
   - **Long-running `ty start`** (native development and tests): a
     `@Schedule` worker in `server/workers/` gets a service token from
     `kit/oidc` (below) and POSTs to its own service's route on the local
     origin. The worker holds no logic beyond that call.
   - **Lambda (Floci, AWS later):** an EventBridge schedule calls the same
     route through the gateway with a service token (slice P1 owns the
     schedules and the small trigger that fetches the token).
3. **Drain on write** stays: a producing route calls `RelayDocument` right
   after its own write, so most events leave within the same request; the
   schedule only sweeps what is left. A push to a FYLO-owning service that
   is busy (reserved concurrency 1) is answered 429 at once and stays
   pending; it never waits, because drain on write can call back into the
   service that is pushing (messaging -> mail-out -> messaging). The Floci
   router does the same (`deploy/floci/README.md`, router limits).
4. **A handler never calls back into the producer of its event.** The
   producer holds its one writer slot while it pushes (drain on write and
   the relay alike), so a synchronous call from the handler back to the
   producer waits for that slot: with the Floci router until the push times
   out, in AWS a throttle that fails every retry. An event therefore
   carries what its consumer needs from the producer: `email.received`
   carries the email's content (section 7), and `messaging` asks `mail-in`
   for it only for an older event that has none.

**Service tokens** (`kit/oidc`, documented in
`docs/mesail/SERVICE-TOKENS.md`; changed by ADR 0005 on 2026-11-11):
- A service asks `identity` (`POST /api/v1/identity/service-token`, shared
  secret `YON_SERVICE_CLIENT_SECRET`) for a token for the domain the call
  is about. So on the receiving side `Caller.DomainId` IS the target
  domain, and SLICES rule 11 holds unchanged: a receiver refuses a
  `Service` token whose `DomainId` differs from the domain in the path, the
  body or the event envelope (`404`, as for any other domain).
- Calls that are about the whole install, or about a domain that may not be
  hosted (the SMTP edge asking whether a domain is hosted, operator jobs,
  the relay schedules), ask for the domain `*` (`oidc.InstallWide`). Only
  routes documented as install-wide accept such a token.
- The helper: `oidc.ServiceTokens{}.Configured().Authorization(domainId)`
  returns `"Bearer ..."`; tokens are cached per domain in the process's
  temporary folder (file mode 0600) until 10 seconds before they expire;
  the route comes from the app's `oidc.servicetokens` setting; the
  `identity` host is in `outgoing.hosts`. `kit/outbox`'s `TokenSource` is
  the port; `StaticToken` stays for tests.

## 5. Tests other slices use

These types are in the kit copies, so `ty test` units can use them:

- `outbox.MemoryStore` (a `Store` in memory; `FailNextSave` for crash tests).
- `outbox.RecordingPusher` (a `Pusher` that records; `FailFor`, `Recover`).
- `outbox.Published{}.One(fields, type, &payload)` returns the one staged
  envelope of that type and decodes its payload, or a `NotPublished` error.
  Use it as `Assert.IsTrue(err == nil, ...)`.
- `events.NewMemorySeen()` (a `SeenStore` in memory).
- `outbox.StaticToken{Value: "..."}` (a `TokenSource`).

`kit/outbox/TwoApps_test.go` shows the whole path: one app stages an event
and relays it over HTTP to a second app's `/events` route, which applies it
exactly once per `eventId`, also after a redelivery.

## 6. Using the packages in an app

`events` and `outbox` are already in `server/packages/kit.list` of the
apps that publish or consume events (directory, channels, messaging,
mail-in, mail-out, search, realtime, notify), and their copies are
committed. Import `mesail.dev/packages/events` and
`mesail.dev/packages/outbox`. A change to `kit/events` or `kit/outbox`
needs `scripts/kit-sync.sh` and the copies in the same commit.

## 7. Payloads

The Go types are in `kit/events`, one file per type. All ids are wire id strings, times RFC 3339 UTC strings, no `domainId` (the envelope has it). No `omitempty`: every key is always present (empty string / empty list when unset).

- `member.upserted` -> MemberUpserted: `memberId` (the person id, the same in every domain), `displayName`, `primaryAddress` (`<username>@<domain>`), `state` (active|disabled|removed). Published into each affected domain when a member is added, changed or removed, or when the person is renamed, disabled or deleted (ADR 0005). On `removed`: channels removes the member from every channel of the domain (publishing `channel.member.removed`), realtime closes the member's connections in the domain, notify deletes the member's push subscriptions and settings in the domain, messaging and search keep the history.
- `channel.created` -> ChannelCreated: `channelId`, `conversationId` (the channel's one conversation; messaging creates it with this id), `name`, `topic`, `visibility` (public|private), `emailAddress` ("" when none), `createdBy` (member id).
- `channel.member.added` -> ChannelMemberAdded: `channelId`, `conversationId`, `memberId`, `joinedAt`.
- `channel.member.removed` -> ChannelMemberRemoved: `channelId`, `conversationId`, `memberId`.
- `channel.archived` -> ChannelArchived: `channelId`, `conversationId`.
- `message.created` -> MessageCreated: `messageId`, `conversationId`, `conversationKind` (channel|direct), `threadRootId` ("" for a root), `authorKind` (member|otherDomain|external), `authorMemberId`, `authorAddress`, `authorDisplayName`, `body` (Markdown), `subject` (email subject, else ""), `attachmentIds[]`, `fileNames[]` (for search), `isEmail`, `mentionedMemberIds[]` (notify: mention), `memberIds[]` (member participants who can read now; realtime fan-out and notify), `createdAt`.
- `message.edited` -> MessageEdited: `messageId`, `conversationId`, `body` (the new Markdown text, so search can index the edit), `memberIds[]` (readers to fan out to), `editedAt`.
- `message.deleted` -> MessageDeleted: `messageId`, `conversationId`, `memberIds[]`, `deletedAt`.
- `reaction.changed` -> ReactionChanged: `messageId`, `conversationId`, `memberId` (who reacted), `emoji`, `added` (true add, false remove), `memberIds[]`.
- `read-position.moved` -> ReadPositionMoved: `conversationId`, `memberId`, `messageId` (last read), `readAt`.
- `email.send.requested` -> EmailSendRequested (CONTRACTS.md): `messageId`, `from`, `rcpt`, `role` (to|cc|bcc: this rcpt's role), `to[]`, `cc[]` (the visible header lists; never Bcc), `subject`, `inReplyTo`, `references[]`, `attachmentIds[]`, `idempotencyKey`. No body: mail-out reads it from `GET /api/v1/<producer>/messages/{id}/source` (Service). Producers: `messaging`, and `notify` for the email summary (`Catalog.ProducersOf`).
- `email.delivery.changed` -> EmailDeliveryChanged (CONTRACTS.md): `messageId`, `rcpt`, `status` (queued|sent|deferred|bounced|failed), `detail` (no content).
- `email.received` -> EmailReceived (CONTRACTS.md): `inboundId`, `rcpts[]` (only this domain's), `rawBlobKey`, `headers` (EmailHeaders), `authResults` (AuthResults: `spf`, `dkim`, `dmarc`, and `handoff`, true when another hosted domain on this install handed the message over; then the DNS results are empty, not fake passes), `content` (EmailContent; missing only in events staged before 2026-11-12).
  - EmailContent: `text`, `bodyFormat`, `blocks[]` (richtext blocks, as in InboundContent), `truncated`, `attachments[]` (EmailAttachment: `fileName`, `mediaType`, `size`, `blobKey`, `contentId?`). The same values as `GET /api/v1/mail-in/messages/{inboundId}/content`, and at most 256 KiB of JSON (`kit/mailin` ContentBudget), because a FYLO document must fit FYLO's 1 MiB request frame: the content is stored in mail-in's `inbound-content` and, escaped, in the staged event of `inbound-raw`, and messaging stores its text and blocks in the Message. Steps: `text` is cut at 200,000 characters; content still over 256 KiB keeps its first 64 KiB of text with blocks made from that text (`bodyFormat: text`); content still over 256 KiB (only with very many or very long attachment names) becomes the unreadable form: empty `blocks` and `attachments`, the first 16 KiB of text. Any cut sets `truncated: true`, and the client then offers the original, which keeps every part.
  - EmailHeaders: `messageId`, `inReplyTo`, `references[]`, `subject`, `from`, `to[]`, `cc[]`.
  - AuthResults: `spf`, `dkim`, `dmarc`, `handoff`.
- `presence.changed` -> PresenceChanged: `memberId`, `state` (active|idle|dnd|offline), `doNotDisturbUntil` ("" when none), `lastSeenAt`.

A slice that needs another payload field changes the type in `kit/events`
through I2 (or its successor) and updates this table in the same commit.
