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

Every FYLO document that can carry events has four more fields (FYLO
rejects arrays of objects, so each pending event is one JSON string):

| Field | Type | Meaning |
| --- | --- | --- |
| `pendingEvents` | string[] | `outbox.PendingEvent` as JSON: the envelope, the subscribers still waiting, attempts, next attempt time, last error |
| `deadEvents` | string[] | `outbox.DeadLetter` as JSON: one per event and subscriber that gave up |
| `outboxPending` | int | number of pending events; the relay finds documents with `outboxPending > 0` (indexed numeric range) |
| `outboxDead` | int | number of dead letters |

The entity model in `server/models/` declares these four fields with these
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
    "retry": { "baseDelaySeconds": 5, "maxDelaySeconds": 900, "maxAttempts": 12 }
  },
  "outgoing": { "hosts": ["127.0.0.1"] }
}
```

- A subscriber gets an event only when `Catalog.ConsumersOf(type)` names it
  and it is listed here. A consumer that is not listed is not wired in this
  deployment and is skipped (the event is not kept for it).
- Every subscriber host must also be in `outgoing.hosts` (Tachyon's egress
  guard refuses other hosts).
- Read the settings with
  `outbox.SettingsReader{}.FromApplicationJSON(os.Getenv("YON_APPLICATION_JSON"))`.
- Retry: the delay is `baseDelay * 2^(attempts-1)`, at most `maxDelay`.
  After `maxAttempts` the remaining subscribers become dead letters. A `4xx`
  answer (except `408` and `429`) is permanent and dead-letters at once.

### 2.4 Role files

| File | What it does |
| --- | --- |
| `server/services/OutboxStoreService.go` (`@Service`, `@Needs dataSource DataSource`) | implements `outbox.Store`: `Pending(limit)` scrolls the service's collections with `outboxPending > 0`; `Load(ref)` reads one document's four fields; `Save(ref, fields)` patches exactly those four fields |
| `server/services/OutboxRelayService.go` (`@Service`, `@Needs` the store service) | builds `outbox.NewRelay(store, outbox.NewHTTPPusher(tokens, timeout), settings, time.Now)` and exposes `RunOnce` and `RelayDocument` |
| `server/controllers/OutboxController.go` (`@RequestRoute("/api/v1/<service>/outbox")`, `@Allow(Service)`) | `POST /relay` runs `RunOnce` and returns `outbox.Report`; `GET /dead` lists dead letters; `POST /dead/requeue` moves one back (`DeadLetters.Requeue`) |
| `server/workers/outbox-relay.go` (`@Schedule({"fixedDelaySeconds":60})`) | the sweep while the service runs as a long-lived `ty start` process (see section 4) |

`Pending` must know the collections to scan: they are split by domain
(`kit/collections` `CollectionName{}.ForDomain(base, domainId)`, for
example `channels-a379a6f6eeafb9a5`). Keep them in a small install-wide collection,
`outbox-collections`, and add a collection name to it before the first
document is written into that collection. Collection names come only from
the service's own code, never from a request.

The service token for pushes comes from I1 (Keycloak client credentials,
realm role `mesail-service`) through `outbox.TokenSource`.

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
   schedule only sweeps what is left.

**Service tokens** (slice I1 builds them in `kit/oidc`, documented in
`docs/mesail/SERVICE-TOKENS.md`; decided with I1 on 2026-11-10):
- A service asks the realm of the domain the call is about: client
  credentials with the confidential client `mesail-service` of that realm
  (one shared secret, `YON_SERVICE_CLIENT_SECRET`, set in every realm like
  `mesail-identity`'s). So on the receiving side `Caller.DomainId` IS the
  target domain, and SLICES rule 10 holds unchanged: a receiver refuses a
  `Service` token whose `DomainId` differs from the domain in the path, the
  body or the event envelope (`404`, as for any other domain).
- Calls that are about the whole install, or about a domain that may not be
  hosted (the SMTP edge asking whether a domain is hosted, operator jobs),
  use the realm `mesail-operators`, which also has a `mesail-service`
  client. Only routes documented as install-wide accept such a token.
- The helper: `oidc.ServiceTokens{}.Configured().Authorization(realm)`
  returns `"Bearer ..."`; tokens are cached per realm in the process's
  temporary folder (file mode 0600) until 10 seconds before they expire;
  the token endpoint comes from the app's `oidc.keycloak` setting; the
  Keycloak host is in `outgoing.hosts`. `kit/outbox`'s `TokenSource` is the
  port; `StaticToken` stays for tests.

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

- `member.upserted` -> MemberUpserted: `memberId` (Keycloak sub), `displayName`, `primaryAddress`, `state` (active|disabled).
- `channel.created` -> ChannelCreated: `channelId`, `conversationId` (the channel's one conversation; messaging creates it with this id), `name`, `topic`, `visibility` (public|private), `emailAddress` ("" when none), `createdBy` (member id).
- `channel.member.added` -> ChannelMemberAdded: `channelId`, `conversationId`, `memberId`, `joinedAt`.
- `channel.member.removed` -> ChannelMemberRemoved: `channelId`, `conversationId`, `memberId`.
- `channel.archived` -> ChannelArchived: `channelId`, `conversationId`.
- `message.created` -> MessageCreated: `messageId`, `conversationId`, `conversationKind` (channel|direct), `threadRootId` ("" for a root), `authorKind` (member|otherDomain|external), `authorMemberId`, `authorAddress`, `authorDisplayName`, `body` (Markdown), `subject` (email subject, else ""), `attachmentIds[]`, `fileNames[]` (for search), `isEmail`, `mentionedMemberIds[]` (notify: mention), `memberIds[]` (member participants who can read now; realtime fan-out and notify), `createdAt`.
- `message.edited` -> MessageEdited: `messageId`, `conversationId`, `body` (the new Markdown text, so search can index the edit), `memberIds[]` (readers to fan out to), `editedAt`.
- `message.deleted` -> MessageDeleted: `messageId`, `conversationId`, `memberIds[]`, `deletedAt`.
- `reaction.changed` -> ReactionChanged: `messageId`, `conversationId`, `memberId` (who reacted), `emoji`, `added` (true add, false remove), `memberIds[]`.
- `read-position.moved` -> ReadPositionMoved: `conversationId`, `memberId`, `messageId` (last read), `readAt`.
- `email.send.requested` -> EmailSendRequested (CONTRACTS.md): `messageId`, `from`, `rcpt`, `role` (to|cc|bcc: this rcpt's role), `to[]`, `cc[]` (the visible header lists; never Bcc), `subject`, `inReplyTo`, `references[]`, `attachmentIds[]`, `idempotencyKey`. No body: mail-out reads it from `GET /api/v1/messaging/messages/{id}/source` (Service).
- `email.delivery.changed` -> EmailDeliveryChanged (CONTRACTS.md): `messageId`, `rcpt`, `status` (queued|sent|deferred|bounced|failed), `detail` (no content).
- `email.received` -> EmailReceived (CONTRACTS.md): `inboundId`, `rcpts[]` (only this domain's), `rawBlobKey`, `headers` (EmailHeaders), `authResults` (AuthResults: `spf`, `dkim`, `dmarc`, and `handoff`, true when another hosted domain on this install handed the message over; then the DNS results are empty, not fake passes).
  - EmailHeaders: `messageId`, `inReplyTo`, `references[]`, `subject`, `from`, `to[]`, `cc[]`.
  - AuthResults: `spf`, `dkim`, `dmarc`, `handoff`.
- `presence.changed` -> PresenceChanged: `memberId`, `state` (active|idle|dnd|offline), `doNotDisturbUntil` ("" when none), `lastSeenAt`.

A slice that needs another payload field changes the type in `kit/events`
through I2 (or its successor) and updates this table in the same commit.
