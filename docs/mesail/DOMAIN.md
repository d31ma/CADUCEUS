# Mesail domain model

- Status: Draft for implementation
- Date: 2026-11-10
- Spec: [SPEC.md](SPEC.md). Architecture: [ADR 0004](../adr/0004-mesail-go-serverless-keycloak.md).

This file names the entities, who owns each one, and the rules that must
always hold. The Go wire models are in `apps/<service>/server/models/`. The
shared rules are in `kit/`: `kit/oidc` (who calls), `kit/recipients`
(recipient kinds and allowed deliveries), `kit/events` (event envelope and
types) and `kit/ids` (wire ids). Terms are defined in SPEC.md section 2.

**The tenant is the mail domain.** Each hosted mail domain (for example
`example.com`) is also its own messaging space, one to one. There is no
organization and no workspace above it. Members of one domain can chat only
with members and channels of the same domain. Email can be sent and received
between domains, but chat messages cannot.

## 1. Services and the data they own

Each service owns its collections. No service reads or writes another
service's collections. Services talk through HTTP contracts
([CONTRACTS.md](CONTRACTS.md)) and events (section 4). Each service is its
own TACHYON Go app in `apps/<service>/`, with the route prefix
`/api/v1/<service>/`.

| Service | Owns (FYLO collections) | Does not own |
| --- | --- | --- |
| `identity` | none (stateless token check) | users and credentials (Keycloak owns them) |
| `directory` | `domains`, `members`, `addresses` (aliases are addresses) | DKIM private keys (secret store), Keycloak realms (Keycloak owns them) |
| `channels` | `channels`, `channel-members` | messages |
| `messaging` | `conversations`, `participants`, `messages`, `reactions`, `read-positions`, `deliveries`, `email-threads` | raw email bytes, attachments |
| `attachments` | `attachments` (metadata) and the blob bucket | message bodies |
| `realtime` | `connections`, `presence` | messages |
| `notify` | `notifications`, `push-subscriptions`, `notify-settings` | messages |
| `search` | `search-index` | the source documents |
| `mail-in` | `inbound-raw` (metadata) and the raw-mail bucket | conversations |
| `mail-out` | `outbound-queue`, `suppressions`, `delivery-attempts` | conversations |

### Roles

Roles are Tachyon `@Allow(...)` names, mapped from the token's
`realm_access.roles` claim by `kit/oidc` `RoleMap`:
| Role | Realm role | Can do |
| --- | --- | --- |
| `Member` | `mesail-member` | use chat and email in their own domain |
| `DomainAdmin` | `mesail-admin` | manage aliases and channel email settings in their own domain only |
| `Operator` | `mesail-operator` | add, verify and remove domains on the install |
| `Service` | `mesail-service` | service-to-service calls (events, address lookup, inbound mail) |

Operators sign in to their own realm, `mesail-operators`. It is not a mail
domain: it has no addresses, members or chat, and it can never be created as
a domain (a domain name always contains a dot). Operator routes never use the
caller's `domainId` as a tenant; they name the target domain in the path.

## 2. Entities

Rules that come from FYLO 26.45.07 (see `research-fylo.md` in the planning
notes):

- **Ids.** A document id is the FYLO TTID (time-ordered) returned on insert.
  The prefixes below (`msg_`, `cnv_`, ...) are added on the wire only (`kit/ids`), so a
  caller cannot pass one kind of id where another is expected. Lookups by a
  natural key (an address, a `Message-ID` hash) use an `$eq` query on an
  indexed field. Two ids are not TTIDs: a `domainId` is the domain name, and
  a member id is the Keycloak `sub`.
- **No arrays of objects.** FYLO rejects them. Lists of objects (recipients,
  reactions, attachments) are separate documents or lists of ids.
- **Order is creation order.** FYLO has no ORDER BY. Lists that need
  newest-first are read with a cursor from the end of a time partition.
- **One collection per write.** FYLO has no transaction across collections.
  Every write touches one collection and is idempotent, so a retry after a
  crash is safe. The outbox event is a field of the same document
  (`pendingEvents`) and is relayed afterwards.

### FYLO layout

- **One root per service.** Each service that stores data owns one FYLO
  root. In Lambda the root is the EFS mount `/mnt/data/<service>/`. Every
  function that owns a FYLO root has reserved concurrency 1, so there is
  exactly one writer per root (ADR 0004). No root is ever shared between
  services or between parallel function instances.
- **Collections are split by domain.** A FYLO collection index stops taking
  writes at about 240 MiB, and domains must never mix. So every collection
  name ends with the domain:
  - small collections: `<collection>.<domainKey>`, for example
    `channels.example_com`;
  - large collections, also split by month: `<collection>.<domainKey>.<YYYYMM>`.
    These are `messages`, `deliveries`, `search-index` and `notifications`,
    for example `messages.example_com.202611`.
- **The domain key.** Domain names contain dots, and the dot separates the
  parts of a collection name. So `domainKey` is the `domainId` with every `.`
  replaced by `_` (`example.com` -> `example_com`). A domain name cannot
  contain `_`, so the key maps back to one domain. The slice that builds
  this must check the key against FYLO's rules for collection names
  (allowed characters and length) and add a test with a long domain name.
- **Install-wide collections.** Only `directory/domains` (the list of hosted
  domains) is not split by domain.
- **Path to scale.** Because each domain already has its own collections, a
  domain (or a set of domains) can later move to its own FYLO root and its
  own function without changing any document.

Every document carries `domainId`. Times are RFC 3339 UTC strings.

### Domain (`directory/domains`)
A hosted mail domain. It is the tenant and the chat boundary.
| Field | Type | Rule |
| --- | --- | --- |
| `domainId` | string | The domain name in lowercase, IDNA A-label (for example `example.com`). The document id. Unique on the install. It is also the Keycloak realm name, so the token issuer `https://<host>/realms/<domainId>` names the domain with no lookup (`kit/oidc` `Realm`, `Caller.DomainId`). |
| `name` | string | The domain name as shown. Same value as `domainId`. |
| `issuer` | string | Keycloak issuer URL of the domain's realm. Unique. |
| `state` | `pending` \| `active` \| `disabled` | mail is accepted only when `active` |
| `dkimSelector` | string | the private key lives in the secret store under `dkim/<domain>/<selector>` |
| `dns` | object | last check result per record: `mx`, `spf`, `dkim`, `dmarc` -> `ok` \| `missing` \| `wrong` |
| `limits` | object | attachment size, message size, retention days |

Only an Operator adds or removes a domain. Creating a domain also creates
its Keycloak realm.

### Member (`directory/members`)
| Field | Type | Rule |
| --- | --- | --- |
| `id` | string | The Keycloak `sub` (user id). With `domainId` it identifies the member everywhere, so no service needs a lookup to know who calls (`kit/oidc` `Caller`). |
| `domainId` | string | the realm the member signs in to. A member belongs to exactly one domain. |
| `displayName` | string | from the token, updated on each sign-in |
| `primaryAddress` | string | an address in the member's own domain |
| `state` | `active` \| `disabled` | mirrored from Keycloak |

### Address (`directory/addresses`)
The routing table for every hosted address.
| Field | Type | Rule |
| --- | --- | --- |
| `address` | string | lowercase, the document id. Unique on the install. Its domain part is always `domainId`. |
| `domainId` | string | |
| `kind` | `member` \| `alias` \| `channel` | |
| `targetId` | string | member id or channel id, always in the same domain |

To look up an address, the service takes the domain part of the address
as the `domainId` and reads `addresses.<domainKey>`. A domain part that is
not in `domains` means the address is not hosted.

### Recipient kinds (`kit/recipients`)
The server decides the kind of every recipient, from the sender's domain
(`Kinds{}` and `Policy.Check` in `kit/recipients`):
| Kind | Meaning | Allowed deliveries | Default |
| --- | --- | --- | --- |
| `member` | a member (or alias) in the sender's domain | `chat`, `email`, `both` | `chat` |
| `channel` | a channel in the sender's domain | `chat` | `chat` |
| `otherDomain` | an address hosted on this install, but in another domain | `email` | `email` |
| `external` | an address not hosted on this install | `email` | `email` |

For `otherDomain` the directory returns only the address and the kind, never
a display name or member id from the other domain.

### Channel, ChannelMember (`channels/...`)
`Channel{id ch_, domainId, name, topic, visibility: public|private,
emailAddress?, archived, conversationId, createdBy}`.
`ChannelMember{channelId, memberId, joinedAt, muted}`.
A channel belongs to one domain; there is no workspace. Every channel member
and the `emailAddress` are in the channel's domain. A channel has exactly
one conversation (`conversationId`), created by `messaging` when the channel
is created.

### Conversation (`messaging/conversations`)
| Field | Type | Rule |
| --- | --- | --- |
| `id` | `cnv_...` | |
| `domainId` | string | |
| `kind` | `channel` \| `direct` | |
| `channelId` | string | set when `kind = channel` |
| `subject` | string | for email conversations, the subject of the first email |
| `hasExternal` | bool | true when any participant is reached by email only (`otherDomain` or `external`). Such a conversation is an *email conversation*. |
| `lastMessageAt` | time | |

### Participant (`messaging/participants`)
| Field | Type | Rule |
| --- | --- | --- |
| `conversationId` | string | |
| `kind` | `member` \| `otherDomain` \| `external` | |
| `memberId` | string | when `member`; the member's `domainId` is the conversation's `domainId` |
| `address` | string | always set; for a member, their address used in this conversation |
| `displayName` | string | |
| `role` | `to` \| `cc` | how they joined; Bcc recipients are never participants |

For a channel conversation the participants are the channel members, read
from `channels` through events, plus email senders into the channel.

### Message (`messaging/messages`)
| Field | Type | Rule |
| --- | --- | --- |
| `id` | `msg_...` | |
| `domainId`, `conversationId` | string | |
| `threadRootId` | string | empty for a root. Threads are one level deep. |
| `author` | `{kind, memberId?, address, displayName}` | `kind` is `member`, `otherDomain` or `external` |
| `clientMessageId` | string | unique per author; makes send idempotent |
| `body` | string | Markdown source, max 40 000 chars |
| `attachmentIds` | []string | |
| `email` | `EmailMeta?` | set when the message was sent or received as email |
| `editedAt`, `deletedAt` | time? | |
| `replyCount`, `lastReplyAt` | int, time | on roots only |
| `createdAt` | time | |

`EmailMeta{messageIdHeader, inReplyTo, references[], subject, from, to[],
cc[], authResults{spf, dkim, dmarc}, suspicious, rawId}`. Bcc is stored only
on the outbound queue item, never on the message.

### Delivery (`messaging/deliveries`)
One row per recipient per message.
| Field | Type | Rule |
| --- | --- | --- |
| `messageId` | string | |
| `recipient` | address | |
| `channel` | `chat` \| `email` | `chat` only for a `member` or `channel` recipient |
| `status` | chat: `delivered` \| `read`; email: `queued` \| `sent` \| `deferred` \| `bounced` \| `failed` | |
| `detail` | string | bounce reason, no message content |

### EmailThreadKey (`messaging/email-threads`)
Maps a `Message-ID` header to `{conversationId, messageId}` in one domain.
Used by inbound threading (AC-IN-3). Document id is the SHA-256 of the
lowercased `Message-ID`.

### Attachment (`attachments/attachments`)
`{id att_, domainId, uploaderId, fileName, mediaType, size, sha256, state:
pending|ready|blocked, blobKey}`. Bytes live in the blob bucket under
`blobKey`, which starts with `<domainId>/`; they never pass through FYLO
documents.

### OutboundItem (`mail-out/outbound-queue`)
`{id out_, domainId (the sender's), messageId, from, rcpt (one per item),
headers, rawBlobKey, attempts, nextAttemptAt, expiresAt, state:
queued|sending|sent|deferred|bounced|expired, idempotencyKey}`.

### Suppression (`mail-out/suppressions`)
`{domainId, address, reason, createdAt}`. Document id is
`sha256(address)` inside `suppressions.<domainKey>`, so lists never mix
domains.

### InboundRaw (`mail-in/inbound-raw`)
`{id in_, domainId, receivedAt, envelopeFrom, rcpts[], rawBlobKey, size,
authResults, state: accepted|routed|failed}`. One email with recipients in
two hosted domains gives one `InboundRaw` per domain, each with only that
domain's recipients and its own raw copy under `<domainId>/`.

### Presence (`realtime/presence`)
`{domainId, memberId, state: active|idle|dnd|offline, dndUntil?, lastSeenAt}`.
Presence is shown only to members of the same domain.

### Notification (`notify/notifications`)
`{id ntf_, domainId, memberId, kind: mention|dm|thread_reply|email,
conversationId, messageId, createdAt, readAt?, pushedAt?, summarizedAt?}`.

### SearchDoc (`search/search-index`)
`{id (= messageId), domainId, conversationId, authorAddress, text, subject,
fileNames[], hasAttachment, isEmail, createdAt}`. Readers are checked at
query time against current membership (AC-SR-2), never against a copy taken
at index time. A search reads only the caller's domain.

## 3. Invariants

1. **Domain first.** Every store method takes a `domainId`; there is no query
   without it. For a signed-in request it is the caller's domain, taken from
   the verified token issuer (`Caller.DomainId`). For SMTP and for an address
   lookup it is the domain part of the recipient address. A caller-supplied
   `domainId` is never trusted. The only exception is the install-wide
   `directory/domains` list.
2. **Chat never crosses a domain.** Conversations, messages, reactions, read
   positions, presence, search hits, channels and channel members belong to
   one domain and are never read or written from another. Every member
   participant of a direct conversation has the conversation's `domainId`.
   Data in another domain answers `404`, as if it did not exist.
3. **Email crosses domains.** A member may email any address, hosted or not.
   An email between two hosted domains still goes through `mail-out` and
   `mail-in` as email. It may be handed over inside the install, without the
   internet, but it gets full headers and a DKIM signature, it is checked on
   receipt, and each side has its own email conversation.
4. **Membership decides visibility.** A member can read a conversation only
   while they are a participant (direct) or a channel member (channel), or
   the channel is public and in their own domain.
5. **Recipient resolution is the server's.** The server decides each
   recipient's kind (section 2, Recipient kinds) and checks the wanted
   delivery with `kit/recipients` `Policy.Check`. `chat` is possible only for
   `member` and `channel` kinds. Everything else is `email` (SPEC AC-CP-2,
   AC-CP-5).
6. **One body, many deliveries.** A message is stored once. Chat and email
   deliveries refer to it; attachments are stored once per domain.
7. **Durable before acknowledge.** SMTP `250` after the raw message and
   `InboundRaw` are durable. A queue item leaves only with a final state.
8. **Idempotency.** `(authorId, clientMessageId)` and outbound
   `idempotencyKey` are unique.
9. **Bcc is private.** Bcc never appears in participants, deliveries visible
   to others, chat events, or headers of other recipients' copies.
10. **No secrets in documents.** DKIM keys and Keycloak client secrets live
    in the platform secret store.

## 4. Events

Services publish events after their own write is durable (outbox pattern:
the event is written in the same FYLO write as the change, then relayed).
Every event uses the `kit/events` `Envelope{eventId, type, domainId,
producer, occurredAt, payload}`; the type names are in `kit/events` `Types`.
Consumers are idempotent by `eventId`, and write only into the envelope's
`domainId`. Event payloads carry ids, not message bodies, except
`message.created`, which `realtime` and `search` need.

| Event | Producer | Consumers |
| --- | --- | --- |
| `member.upserted` | directory | messaging, notify, search |
| `channel.created`, `channel.member.added`, `channel.member.removed`, `channel.archived` | channels | messaging, realtime, search |
| `message.created`, `message.edited`, `message.deleted` | messaging | realtime, notify, search |
| `reaction.changed`, `read-position.moved` | messaging | realtime, notify |
| `email.send.requested` | messaging | mail-out |
| `email.delivery.changed` | mail-out | messaging |
| `email.received` | mail-in | messaging |
| `presence.changed` | realtime | realtime (fan-out within the domain) |
