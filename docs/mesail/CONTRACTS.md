# Mesail API contracts

- Status: Draft for implementation
- Date: 2026-11-10
- Spec: [SPEC.md](SPEC.md). Domain: [DOMAIN.md](DOMAIN.md).

The Go wire models are the contract. Each one is a file in
`apps/<service>/server/models/` with the same name as the type (one type per
file, data only, `json` tags in lowerCamel). They exist on `develop` now;
a slice that changes one updates this file in the same commit. A Tac app
keeps its own copies of the models it uses in `client/models/`. The tables below list every
route, its caller rule (Tachyon `@Public`, `@SignedIn`, `@Allow(Role)`), its
request and response model, and its errors. A `?` after a field means the
field may be empty or left out.

## Conventions

- Base path `/api/v1/<service>/...`. The services are `identity`,
  `directory`, `channels`, `messaging`, `attachments`, `realtime`, `notify`,
  `search`, `mail-in` and `mail-out`, each its own TACHYON Go app in
  `apps/<service>/`. Each service owns its prefix, so a gateway (or CDN path
  rule) can route by prefix to an independently deployed service.
- **The domain is the tenant.** Every hosted mail domain is one Keycloak
  realm and one chat space (DOMAIN.md). The caller's `domainId` is the realm
  name in the token issuer `https://<host>/realms/<domainId>` (`kit/oidc`
  `Realm`, `Caller.DomainId`). No route takes the caller's domain from the
  request. Chat data never crosses a domain; email does.
- Every non-public route takes `Authorization: Bearer <access token>` from
  Keycloak. The token is checked by the service's `IdentityProvider`
  (`apps/<service>/server/controllers/TokenIdentityProvider.go`), which uses
  the shared verifier package `kit/oidc`. Roles (empty structs in
  `apps/<service>/server/roles/`) come from the token's `realm_access.roles`
  claim (`kit/oidc` `RoleMap`):
  - `Member` from `mesail-member`;
  - `DomainAdmin` from `mesail-admin`, acting only in its own domain;
  - `Operator` from `mesail-operator`. Operators sign in to the operator
    realm `mesail-operators`, which is not a mail domain. Operator routes
    name the target domain in the path;
  - `Service` from `mesail-service`, for calls between services.
- Errors use one model, `ApiError{code, message, field?}`. Codes are
  lowercase snake case and stable. Status codes: `400 invalid_request`,
  `401 unauthenticated`, `403 forbidden`, `404 not_found` (also for data in
  another domain or a private channel the caller is not in),
  `409 conflict` / `recipients_changed` / `duplicate`, `413 too_large`,
  `422 chat_not_allowed`, `429 rate_limited`, `503 unavailable`.
- Lists are paged with `?cursor=<opaque>&limit=<1..100>` and return a page
  model `{items: [...], nextCursor: string}` (`MemberPage`,
  `ConversationPage`, `MessagePage`, `NotificationPage`, `SearchPage`).
  `nextCursor` is empty on the last page.
- Every service has `GET /api/v1/<service>/health` (`@Public`) returning
  `Health{service, status: ok|degraded}`.
- Every service that consumes events has `POST /api/v1/<service>/events`
  (`Allow(Service)`) taking the `kit/events` `Envelope{eventId, type,
  domainId, producer, occurredAt, payload}` (slice I2) and answering `200
  EventReceipt{eventId, status: applied|duplicate}`. The consumer writes
  only into the envelope's `domainId`. A refused event answers `400`
  (bad envelope, wrong producer, type not consumed) or `403` (not a
  service); a storage failure answers `503` so the relay retries. Details:
  [EVENTS.md](EVENTS.md).
- Every service that publishes events has `POST /api/v1/<service>/outbox/relay`
  (`Allow(Service)`, answers `OutboxReport{documents, delivered, retried,
  deadLettered}`), called by the relay schedule (EVENTS.md section 4).

## identity (stateless)

Keycloak is reached only from the backend (the owner's rule). The browser
never talks to Keycloak.

| Route | Rule | Request | Response |
| --- | --- | --- | --- |
| `POST /api/v1/identity/session` | Public | `SignInRequest{address, password, oneTimeCode?}` | `200 Session{me, accessToken, expiresIn (seconds), sessionKey}`; `401 unauthenticated` (one message for every failure); `429 rate_limited` |
| `POST /api/v1/identity/session/refresh` | Public | `RefreshRequest{sessionKey}` | `200 Session`; `401` when the Keycloak session has ended |
| `DELETE /api/v1/identity/session` | Public | `RefreshRequest{sessionKey}` | `204`; ends the Keycloak session |
| `GET /api/v1/identity/me` | SignedIn | - | `Me{memberId, domainId, displayName, primaryAddress, roles[]}` |

How it works:
- `POST /session` takes the domain from `address` (the realm has the same
  name) and calls that realm's token endpoint server to server as the
  confidential client `mesail-identity` (direct access grants). Every
  realm uses the same client secret, `YON_IDENTITY_CLIENT_SECRET` in
  `identity`'s `.env`; `directory` sets that value when it creates a realm.
  Rotating it is an operator step (all realms, then `.env`, then restart).
- It answers the Keycloak access token (a JWT, 60 seconds) as
  `accessToken`, and the refresh token encrypted with AES-GCM under
  `YON_IDENTITY_SESSION_KEY` as `sessionKey`. identity keeps no state; a
  `sessionKey` stops working when Keycloak ends the session (sign-out, user
  disabled, idle 30 minutes, at most 10 hours).
- Tac keeps `accessToken` in memory only and sends it as
  `Authorization: Bearer <accessToken>` on every Yon call (a
  `// @RequestHeader("Authorization")` parameter). It keeps `sessionKey` in
  a `// @SessionState` field (per tab, gone when the tab closes). When a
  call answers `401`, Tac calls `POST /session/refresh` once and retries;
  when that fails, it shows the sign-in form. No timer is needed.
- Why not cookies: TACHYON 26.46.02's Tac HTTP client always sends requests
  with `credentials: 'omit'`, so it sends no cookie and the browser ignores
  `Set-Cookie` on its responses. When TACHYON offers same-origin
  credentials, the `sessionKey` moves to an `HttpOnly` cookie (requested in
  ADR 0004).
- Every Yon app's `TokenIdentityProvider` checks the Bearer token with
  `kit/oidc` `Gate`, for browser and service-to-service calls alike.
- Realtime cannot send a header from the browser: `realtime` takes the
  access token in the first socket message (`{type:"auth", token}`), and an
  event stream takes a single-use ticket from
  `POST /api/v1/realtime/tickets` (SignedIn, 30 seconds). Never a token in a
  URL.
- Second factor: TOTP through Keycloak's direct-grant conditional OTP
  (`oneTimeCode`). Passkeys are not available in this flow; they are a
  later slice.
- Brute force: Keycloak brute-force detection is on in every realm (per
  user), and the gateway or dev router limits requests per client address
  (slice P1). `identity` keeps no counter; it answers `429` only when
  Keycloak reports the account as temporarily locked.
  Passwords, codes, tokens and session keys never appear in logs.
- A person with accounts in two domains signs in once per domain; the domain
  switcher keeps one `Session` per domain in the same `@SessionState`.

Account settings (the owner's choice: password and 2-step code are managed
in Mesail's own settings; the browser still never talks to Keycloak):

| Route | Rule | Request | Response |
| --- | --- | --- | --- |
| `POST /api/v1/identity/password` | SignedIn | `PasswordChange{currentPassword, newPassword, oneTimeCode?}` | `200 Session` (fresh tokens); `401` wrong current password or code; `400 password_policy` with `ApiError.message` naming the failed rule |
| `GET /api/v1/identity/totp` | SignedIn | - | `200 TotpStatus{enabled}`: whether the caller's account has an OTP credential (read through the `mesail-account` client; `Me` stays token-only) |
| `POST /api/v1/identity/totp/setup` | SignedIn | `TotpSetupRequest{currentPassword}` (no `oneTimeCode`: setup is only for an account without a 2-step code; to replace a code, `DELETE /totp` first) | `200 TotpSetup{secret, otpauthUri, qrImage, setupKey}`; `401` wrong password, or the account already has a 2-step code |
| `POST /api/v1/identity/totp/confirm` | SignedIn | `TotpConfirm{setupKey, code}` | `204`; `400 invalid_code` |
| `DELETE /api/v1/identity/totp` | SignedIn | `TotpRemove{currentPassword, oneTimeCode}` | `204`; `401` |

- **Checking the current password:** a direct grant to the member's realm
  with the current password (and code). Keycloak's brute-force detection
  counts it like a sign-in. The session from that grant is KEPT: it becomes
  the caller's fresh `Session`, because Keycloak does not accept the same
  one-time code twice (agreed with slice I1).
- **Changing:** through the Keycloak admin API, signed in as the
  confidential client `mesail-account` of the member's realm, whose service
  account has only the realm-management roles it needs (`manage-users`,
  `view-users`); secret `YON_IDENTITY_ACCOUNT_SECRET`, the same in every
  realm, set by `directory` when it creates a realm. identity changes only
  the user whose `sub` is in the caller's token; it never takes a user id
  from the request.
- **Password:** `PUT /admin/realms/<domain>/users/<sub>/reset-password`
  (`temporary: false`), then `GET .../users/<sub>/sessions` and every
  session except the one from the check (its `sid` claim) is deleted; the
  caller gets that session as its fresh `Session`. The admin base URL is
  the app setting `oidc.admin`.
- **2-step code:** `setup` makes a random 160-bit TOTP secret (SHA-1, 6
  digits, 30 s, as Keycloak's default OTP policy), and returns it as text,
  as an `otpauth://` URI, as a QR code image (`qrImage`, a `data:image/png`
  URI drawn on the server), and as `setupKey`: the secret encrypted with
  `YON_IDENTITY_SESSION_KEY` and bound to the caller's `sub` and a 10-minute
  expiry. identity keeps no state. `confirm` decrypts `setupKey`, checks
  the code (one step of clock drift allowed), and only then stores the OTP
  credential in Keycloak. `DELETE` removes the user's OTP credential after
  checking password and code.
- Keycloak's stock admin API cannot always create an OTP credential with a
  given secret. If it cannot, `mesail-keycloak` adds a narrow admin endpoint
  in its provider (set the OTP credential of one user, callable only by
  `mesail-account`); slice I1 and `mesail-keycloak` settle which one.
- Limits and logs: the per-IP limit at the gateway (P1) covers these routes;
  Keycloak brute-force detection covers the password checks. Passwords,
  codes, secrets, `setupKey` and `qrImage` never appear in logs. Every
  failure answers without saying which part was wrong, except
  `password_policy`.

## directory

| Route | Rule | Request | Response |
| --- | --- | --- | --- |
| `POST /api/v1/directory/recipients/resolve` | SignedIn | `ResolveRequest{addresses[] (max 100)}` | `ResolveResponse{recipients[]: ResolvedRecipient}` |
| `GET /api/v1/directory/members?query=` | SignedIn | - | `MemberPage` of `MemberSummary{memberId?, displayName, primaryAddress}`, only members of the caller's domain |
| `GET /api/v1/directory/members/{memberId}` | SignedIn | - | `MemberSummary`; `404 not_found` when the member is unknown or in another domain (used by `channels` for invites) |
| `GET /api/v1/directory/domains` | Allow(Operator) | - | `[]Domain` |
| `GET /api/v1/directory/domains/{domainId}` | Allow(Operator, Service) | - | `Domain` or `404` (used by `mail-out` for `dkimSelector` and the hosted check) |
| `POST /api/v1/directory/domains` | Allow(Operator) | `CreateDomainRequest{name}` | `201 Domain` (also creates the domain's Keycloak realm) |
| `POST /api/v1/directory/domains/{domainId}/verify` | Allow(Operator) | - | `Domain` (with `dns` states) |
| `DELETE /api/v1/directory/domains/{domainId}` | Allow(Operator) | - | `204` |
| `POST /api/v1/directory/aliases` | Allow(DomainAdmin) | `CreateAliasRequest{address, memberId?}` | `201 Address` |
| `POST /api/v1/directory/channel-addresses` | Allow(DomainAdmin) | `ChannelAddressRequest{address, channelId}` | `201 Address` with `kind: channel`; the same address for the same channel again answers `201`; `409 duplicate` when a member, alias or other channel has it; `400 invalid_request` (`address`) when it is not in the caller's domain (called by `channels`) |
| `DELETE /api/v1/directory/channel-addresses/{address}` | Allow(DomainAdmin) | - | `204`; `404` unless it is a channel address in the caller's domain |
| `GET /api/v1/directory/addresses/{address}` | Allow(Service) | - | `Address{address, domainId, kind, targetId}`; `404 domain_not_hosted` when the domain is not hosted or not `active`; `404 not_found` for an unknown address in a hosted, active domain (used by the SMTP edge and `messaging`) |

`Domain{domainId, name, state, dkimSelector, dns: DnsState{mx, spf, dkim,
dmarc}}`. `domainId` is the domain name in lowercase (for example
`example.com`) and is also the realm name. `Address.kind` is
`member|alias|channel`. An alias address and its member are both in the
caller's domain.

Operator routes also require a token from the operator realm
`mesail-operators`; an `mesail-operator` role inside a mail domain's realm
answers `403`. A signed-in request from a realm that is not a hosted domain,
or from a `disabled` domain, answers `403`. The `Service` routes
(`addresses/{address}`, `domains/{domainId}`) accept a token from the target
domain's realm or from `mesail-operators`; any other realm answers `404`.
The outbox routes (`POST /api/v1/directory/outbox/relay`,
`GET .../outbox/dead`, `POST .../outbox/dead/requeue`, EVENTS.md) are
install-wide and accept only `mesail-operators`.

`ResolvedRecipient{address, displayName, kind:
member|channel|otherDomain|external, memberId?, channelId?, defaultDelivery:
chat|email, allowedDeliveries[]}`. The kind is decided from the caller's
domain by `kit/recipients`:

| `kind` | Meaning | `allowedDeliveries` | `defaultDelivery` |
| --- | --- | --- | --- |
| `member` | a member or alias in the caller's domain | `["chat","email","both"]` | `chat` |
| `channel` | a channel in the caller's domain | `["chat"]` | `chat` |
| `otherDomain` | hosted on this install, in another domain | `["email"]` | `email` |
| `external` | not hosted on this install | `["email"]` | `email` |

For `otherDomain` the response has no `memberId`, no `channelId` and no
display name from the other domain.

## channels

Channels belong to one domain. There is no workspace.

| Route | Rule | Request | Response |
| --- | --- | --- | --- |
| `GET /api/v1/channels` | SignedIn | - | `[]Channel` (public ones in the caller's domain, plus private ones the caller is in) |
| `POST /api/v1/channels` | SignedIn | `CreateChannelRequest{name, topic?, visibility: public|private}` | `201 Channel` |
| `POST /api/v1/channels/{channelId}/join` | SignedIn | - | `204` |
| `POST /api/v1/channels/{channelId}/leave` | SignedIn | - | `204` |
| `POST /api/v1/channels/{channelId}/invite` | SignedIn | `AddMembersRequest{memberIds[]}` | `204` |
| `PATCH /api/v1/channels/{channelId}` | SignedIn; `topic` for a channel member or `DomainAdmin`; `archived` for the channel creator or `DomainAdmin`; `emailAddress` for `DomainAdmin` | `UpdateChannelRequest{topic?, emailAddress?, archived?}` | `Channel` |

`Channel{channelId, name, topic?, visibility, emailAddress?, archived,
conversationId}`. `channelId` is `ch_<TTID>`; `conversationId` is
`cnv_<the same TTID>`, and `messaging` creates that conversation when it
receives `channel.created`.

- Every route needs the `Member` or `DomainAdmin` role; a caller without
  either (for example an Operator) answers `403 forbidden`.
- A private channel the caller is not in, a channel of another domain, an
  unknown id and an id of another kind all answer `404 not_found`.
- `POST /api/v1/channels`: the name is 1 to 80 characters of `a-z`, `0-9`,
  `-` and `_` (`400`, field `name`), `visibility` is `public` or `private`
  (`400`, field `visibility`), `topic` has at most 250 characters (`400`,
  field `topic`). A name already used in the domain answers `409 conflict`.
  The creator becomes the first member.
- `join` and `leave` are idempotent. `invite` takes 1 to 100 member ids
  (`400`, field `memberIds`); a member id that is not in the caller's domain
  answers `404` and adds nobody. Channels checks each id with directory's
  `GET /api/v1/directory/members/{memberId}`, sending the caller's token.
- An archived channel is read-only: `join`, `invite` and a `topic` change
  answer `409 conflict`, unless the same `PATCH` sets `archived: false`.
  A `PATCH` that archives an open channel may change its topic too.
  `leave` still works.
- A `PATCH` without the right for a field answers `403 forbidden`. A
  channel `emailAddress` must be `local@<the channel's domain>` (`400`,
  field `emailAddress`); an empty string removes it. Channels claims the
  address with directory's `POST /api/v1/directory/channel-addresses` and
  frees the old one with `DELETE /api/v1/directory/channel-addresses/{address}`,
  both with the caller's token; an address that already belongs to a
  member, alias or channel answers `409 duplicate`.
- Events: `channel.created`, `channel.member.added`,
  `channel.member.removed` and `channel.archived`, with the `kit/events`
  payloads (EVENTS.md section 7). Each one is staged in the changed
  document (outbox, slice I2) and pushed right after the write.
- Outbox routes, `Allow(Service)`: `POST /api/v1/channels/outbox/relay`
  runs one sweep of at most 25 documents or 20 seconds and answers
  `SweepReport{documents, delivered, retried, deadLettered, more}` (`more`
  is true when it stopped early); `GET
  /api/v1/channels/outbox/pending` answers `OutboxCount{documents}` (documents with events still waiting, counted up to 100); `GET
  /api/v1/channels/outbox/dead` answers `[]DeadEvent{collection, documentId,
  eventId, type, subscriber, attempts, lastError, deadAt}`; `POST
  /api/v1/channels/outbox/dead/requeue` takes `RetryRequest{collection,
  documentId, eventId, subscriber}` and answers `204` (`404` for an
  unknown collection, document or dead letter).

## messaging

| Route | Rule | Request | Response |
| --- | --- | --- | --- |
| `GET /api/v1/messaging/conversations` | SignedIn | - | `ConversationPage` of `ConversationSummary{conversationId, kind, title, hasExternal, unreadCount, mentionCount, lastMessageAt}` |
| `GET /api/v1/messaging/conversations/{conversationId}/messages` | SignedIn | `?threadRootId=` | `MessagePage` of `Message`, newest first. Without `threadRootId`: the roots of the conversation plus replies sent with `alsoToConversation`. With `threadRootId`: only the replies of that thread, never the root itself |
| `GET /api/v1/messaging/messages/{messageId}` | SignedIn (a participant) | - | `Message`; used for a thread's root and for links to one message |
| `POST /api/v1/messaging/messages` | SignedIn | `SendRequest` | `201 SendResult{message, deliveries[]}` |
| `PATCH /api/v1/messaging/messages/{messageId}` | SignedIn | `EditRequest{body}` | `Message` |
| `DELETE /api/v1/messaging/messages/{messageId}` | SignedIn | - | `204` |
| `GET /api/v1/messaging/messages/{messageId}/original` | SignedIn (a participant) | - | `Download{url, expiresAt}` for the raw email as a `.eml` file; `404` for a chat message. messaging asks `GET /api/v1/mail-in/messages/{inboundId}/original` (`Allow(Service)`) for it. The client opens it as a download link and never renders it. |
| `PUT /api/v1/messaging/messages/{messageId}/reactions/{reaction}` | SignedIn | - | `204` |
| `DELETE /api/v1/messaging/messages/{messageId}/reactions/{reaction}` | SignedIn | - | `204` |
| `PUT /api/v1/messaging/conversations/{conversationId}/read` | SignedIn | `ReadRequest{messageId}` | `204` |
| `GET /api/v1/messaging/attachments/{attachmentId}/access` | SignedIn | - | `204` when the caller can read a message of its domain that carries the attachment, else `404` (called by `attachments` with the caller's own token). messaging answers it from one `attachment-uses` row per attachment of a message, so a message carries at most 10 attachments; more answers `400 invalid_request` with `field: attachmentIds` |

`SendRequest{clientMessageId, conversationId?, threadRootId?,
alsoToConversation, body,
attachmentIds[], recipients[]: RecipientChoice, email?: EmailOptions}`.
`RecipientChoice{address, role: to|cc|bcc, delivery: chat|email|both,
expectedKind: member|channel|otherDomain|external}`.
`EmailOptions{subject, fromAddress?, sendToExternal (for channel threads)}`.
`fromAddress` must be an address of the caller in the caller's domain.
`alsoToConversation` (default false) is allowed only with `threadRootId`:
the reply stays in its thread and is also listed in the conversation, the
"Also send to #channel" option. `Message` carries the same flag, so the
client can show "replied to a thread" with a link to the root.

Rules (SPEC AC-CP):

- A send to an existing conversation without new recipients needs no
  `recipients`. A conversation in another domain answers `404`.
- Chat never leaves the caller's domain. The server re-resolves every
  recipient from the caller's domain and checks the delivery with
  `kit/recipients` `Policy.Check`:
  - if `expectedKind` differs from the fresh kind, it answers
    `409 recipients_changed` with `RecipientsChanged{code, message,
    addresses[]}`, naming the changed addresses; the client resolves them
    again;
  - if `expectedKind` is right but `delivery` asks for chat (`chat` or
    `both`) for a kind that allows email only (`otherDomain`, `external`),
    it answers `422 chat_not_allowed` (`field` names the address);
  - any other delivery that the kind does not allow (email to a `channel`)
    answers `400 invalid_request`.
- An email to an `otherDomain` address is sent as email through `mail-out`,
  like any other email (DOMAIN.md invariant 3).
- When any delivery is `email`, `email.subject` is required for a new
  conversation.

Reactions are identified by a NAME, not an emoji character, because a Tac
path value may only use `A-Z a-z 0-9 . _ ~ -` (found by slice F2). A name
matches `[a-z0-9_-]{1,32}` and must be in the fixed list that `messaging`
accepts (in `kit/messaging`, owned by M3; first release: `thumbs_up`,
`thumbs_down`, `heart`, `laugh`, `surprised`, `sad`, `party`, `eyes`,
`check`, `pray`, `fire`, `rocket`); any other name answers `400
invalid_request` with `field: reaction`. `Message.reactions` and
`myReactions` are keyed by name; the client maps names to glyphs and
accessible labels.

`Message{messageId, conversationId, threadRootId?, author: Author, body,
bodyFormat: markdown|text|email, blocks[]: Block, truncated,
attachments[]: AttachmentRef, email?: EmailSummary, reactions:
map[string]int, myReactions[], replyCount, lastReplyAt?, editedAt?,
deletedAt?, createdAt, deliveries[]: DeliveryStatus}`.
`Author{kind: member|otherDomain|external, memberId?, address,
displayName}`. `AttachmentRef{attachmentId, fileName, mediaType, size}`.
`EmailSummary{subject, from, to[], cc[], suspicious, authResults:
AuthResults{spf, dkim, dmarc}}`. `DeliveryStatus{recipient, channel:
chat|email, status, detail?}`.

## attachments

| Route | Rule | Request | Response |
| --- | --- | --- | --- |
| `POST /api/v1/attachments` | SignedIn | `CreateUploadRequest{fileName, mediaType, size, sha256}` | `201 Upload{attachmentId, uploadUrl, expiresAt}` |
| `POST /api/v1/attachments/{attachmentId}/complete` | SignedIn | - | `Attachment{attachmentId, fileName, mediaType, size, sha256, state}` |
| `GET /api/v1/attachments/{attachmentId}` | SignedIn | - | `Download{url, expiresAt}` (short-lived signed URL) |

An attachment in another domain answers `404`.

- **Upload.** `uploadUrl` is a signed `PUT` link that lives
  `attachments.link.seconds` (default 300). The client sends the raw bytes
  with no `Authorization` header: exactly `size` bytes. Then it calls
  `complete`. The bytes never pass through a Yon route, because TACHYON
  accepts only UTF-8 request bodies of at most 12 MiB.
- **Limits and checks** (AC-MS-5). `size` is 1 byte to the domain limit
  (default 25 MiB, `attachments.limit.bytes`): over it answers `413
  too_large`. `fileName` is 1 to 255 bytes without `/`, `\` or control
  characters; `mediaType` is `type/subtype`; `sha256` is 64 lowercase hex
  characters; otherwise `400 invalid_request` with `field`. The 50 MiB per
  message limit is checked by `messaging` from the `size` of each
  attachment.
- **Complete.** Only the uploader may complete (`404` for anyone else). It
  reads the stored bytes back: no bytes yet answers `409 conflict` (field
  `file`); fewer bytes `409` (field `size`); more bytes `413`; another
  SHA-256 `409` (field `sha256`). A file whose sniffed type contradicts
  `mediaType` (HTML declared as anything else, or an image declared as
  another kind of file) gets `state: blocked` and can never be downloaded.
  Otherwise `state: ready`. Completing again answers the same attachment.
- **Download.** Only for a `ready` attachment (`409 conflict`, field
  `state`, before that). The caller gets the link when it is the uploader,
  a `Service` caller of the same domain, or a member for whom
  `GET /api/v1/messaging/attachments/{attachmentId}/access` answers `2xx`;
  everyone else gets `404`. The `url` is a signed `GET` link that lives
  `attachments.link.seconds` and answers with `Content-Disposition:
  attachment`.
- **Blob links.** With the S3 adapter the links are AWS Signature V4
  presigned URLs (path style, `<public endpoint>/<bucket>/<domainId>/<name>`).
  With the local-folder adapter (all-in-one image, native dev) they are
  `<attachments.link.base>/<domainId>/<name>?expires&type&size&file&signature`
  (HMAC-SHA256), served by `kit/blobs` `LocalHandler` mounted by the router
  at `/blobs` with the same folder and secret as the `attachments` app
  (`PUT` answers `200`; a bad or expired signature `403`; a wrong size
  `400`).

## realtime

| Route | Rule | Request | Response |
| --- | --- | --- | --- |
| `GET /api/v1/realtime/socket` (socket route) | SignedIn | - | server-to-client `RealtimeEvent` |
| `PUT /api/v1/realtime/presence` | SignedIn | `PresenceRequest{state: active|idle|dnd, doNotDisturbUntil?}` | `204` |
| `GET /api/v1/realtime/presence?memberIds=` | SignedIn | - | `[]Presence{memberId, state, doNotDisturbUntil?, lastSeenAt}` |

Presence is per domain: a caller sees presence only of members in its own
domain, and member ids from another domain are left out of the answer. The
socket sends only events of the caller's domain.

`RealtimeEvent{type, conversationId, messageId, memberId?, state}` carries
ids only, never message bodies; the client fetches what it needs.
`RealtimeEvent.type` is one of `message.created`, `message.edited`,
`message.deleted`, `reaction.changed`, `read.moved`, `presence.changed`,
`typing`, `notification`. The socket re-checks the token every 60 seconds
and closes with code 4401 when it is no longer valid (SPEC AC-ID-4).

## notify

| Route | Rule | Request | Response |
| --- | --- | --- | --- |
| `GET /api/v1/notify/notifications` | SignedIn | - | `NotificationPage` of `Notification{notificationId, kind, conversationId, messageId, createdAt, readAt?}` |
| `PUT /api/v1/notify/notifications/{notificationId}/read` | SignedIn | - | `204` |
| `POST /api/v1/notify/push-subscriptions` | SignedIn | `PushSubscriptionRequest{endpoint, p256dh, auth}` | `201` |
| `GET /api/v1/notify/settings` / `PUT` | SignedIn | `NotifySettings{emailSummaryAfterMinutes (0=off, 15..1440), mutedConversationIds[]}` | `NotifySettings` |

## search

| Route | Rule | Request | Response |
| --- | --- | --- | --- |
| `GET /api/v1/search?q=&conversationId=&author=&from=&to=&hasAttachment=&isEmail=` | SignedIn | - | `SearchPage` of `SearchHit{messageId, conversationId, snippet, createdAt}` |
| `POST /api/v1/search/events` | Allow(Service) | `Envelope` | `EventReceipt{eventId, status}` (EVENTS.md section 3; indexes `message.created`, `message.edited` and `message.deleted`, and applies the other subscribed types without change) |

A search reads only the caller's domain, and only conversations the caller
can read now.

Query rules (slice S1):

- `q` has at most 500 characters and 16 words. Every word must start a word
  of the message body, email subject, sender name or address, or attachment
  file name. An empty `q` matches every readable message.
- `from` and `to` take an RFC 3339 time or a date (`2026-11-10`); a date in
  `to` includes the whole day. `author` is an address or a member id.
  `hasAttachment` and `isEmail` take `true` or `false`. Paging uses
  `cursor` and `limit` (1..100, default 20), newest first.
- Bad values answer `400 invalid_request` with `field`. When membership
  cannot be checked, the search answers `503 unavailable` and shows nothing.
- Membership is read at query time with the caller's own token:
  `GET /api/v1/messaging/conversations` (all pages) and `GET /api/v1/channels`.

## Rich text: one block format for chat and email

The client never receives HTML to render. Every message body reaches the
client as typed blocks, made on the server by `kit/richtext` (slice M1
owns it; M3 uses it):

- `Block{kind: paragraph|heading|quote|listItem|code|divider|image|row,
  level, ordered, spans[]: Span, image?: ImageRef, cells[]: string}`.
  `level` is the heading level (1 to 6) or the list or quote depth.
  `row` is one table row as plain cell texts.
- `Span{text, bold, italic, code, strike, href?}`. `href` is kept only for
  `https`, `http` and `mailto`; any other link becomes plain text.
- `ImageRef{alt, attachmentId?, remoteUrl?}`. An inline image of the
  message itself is an attachment; a remote image keeps `remoteUrl`, and
  the client shows a placeholder until the user allows images for that
  sender (SPEC AC-IN-5). Tac pages allow images only from their own origin
  or `data:` URLs (CSP `img-src 'self' data:`), so in the first release an
  allowed remote image opens as a link, and an inline attachment image is
  a named placeholder that downloads through `GET /api/v1/attachments/
  {attachmentId}` (F2 reuses F3's `AttachmentsClient` with `@Needs`).
  Showing them inline needs a same-origin image route, which is a later
  slice.
- `bodyFormat` says where the blocks came from: `markdown` (a chat
  message; `body` is its Markdown source), `text` (a plain-text email;
  `body` is the text), or `email` (an HTML email; `body` is its plain-text
  part, or text made from the HTML).
- Limits: at most 2,000 blocks and 200,000 characters of text per message.
  Longer bodies are cut, `truncated` is true, and the client offers the
  original as a download.
- Conversion rules for HTML email: parse with a real HTML parser; keep only
  the allowlisted elements; drop `script`, `style`, `form`, `iframe`,
  `object`, `embed`, `svg`, `math`, comments, every `on*` attribute and
  every `style` attribute; keep text in reading order; never pass markup
  through. A fuzz test in `kit/richtext` must show that no input produces a
  block with markup or a link outside the allowed schemes.
- `mail-in` converts each received email once and keeps the result. It
  answers `GET /api/v1/mail-in/messages/{inboundId}/content`
  (`Allow(Service)`, a token for the recipient's domain) with
  `InboundContent{text, bodyFormat, blocks[], truncated, attachments[]:
  InboundAttachment{fileName, mediaType, size, blobKey, contentId?}}`.
  `messaging` reads it when it handles `email.received`, and stores the
  blocks on the Message. `messaging` makes the blocks for chat messages
  from their Markdown with the same package.

## mail-in (SMTP, not HTTP)

- SMTP on port 25 (STARTTLS), run as an always-on edge listener (ADR 0004,
  `edge/smtp/README.md`).
- `RCPT TO` is checked against `directory`
  (`GET /api/v1/directory/addresses/{address}`); unknown hosted recipients
  get `550 5.1.1`. Mail for a domain that is not hosted, or not `active`,
  gets `550 5.7.1` (no open relay). The edge tells the two apart by the
  404 body: `ApiError{code:"domain_not_hosted"}` means the domain is not
  hosted or not active, and `ApiError{code:"not_found"}` means an unknown
  address in an active domain. Any other answer (a 404 with another code
  or none, a 5xx, a timeout) gets `451 4.4.3`, so the sender retries.
- The edge checks SPF, DKIM and DMARC (`kit/mailauth`) and parses the
  headers (`kit/mime`). A DMARC failure under a `reject` policy gets
  `550 5.7.1` and nothing is stored (AC-IN-4).
- For each recipient domain, the edge writes its own raw copy (with an
  `Authentication-Results` header added) to the blob store under the
  `kit/blobs` key `<domainId>/inbound-<sha256 hex>`, then calls
  `POST /api/v1/mail-in/messages` (`Allow(Service)`) with
  `InboundEnvelope{envelopeFrom, recipients[] (that domain only, 1..100),
  rawBlobKey (`<owner domain>/<name>`, name `[A-Za-z0-9_-]{1,128}`), size
  (1 byte..64 MiB), remoteAddress, helo, tls, headers:
  InboundHeaders, authResults: AuthResults}`, and answers `250` only after
  every domain answered `201 InboundAccepted{inboundId}`.
  - `InboundHeaders{messageId, inReplyTo, references[], subject, from:
    MailboxAddress, to[]: MailboxAddress, cc[]: MailboxAddress}`;
    `MailboxAddress{name, address}`. Message ids have no angle brackets.
  - `AuthResults{spf, dkim, dmarc, dmarcPolicy, suspicious, handoff}`: the
    result words of RFC 8601 (`pass`, `fail`, `softfail`, `neutral`,
    `none`, `temperror`, `permerror`), the applied DMARC policy, and
    `suspicious` when DMARC, SPF or every DKIM signature failed. The edge
    always sends `handoff: false`.
  - **Internal hand-off** (AC-DM-6, from `mail-out`): `handoff: true`, no
    DNS checks and no invented `pass` values. `rawBlobKey` is under the
    SENDER's domain (the domain of `envelopeFrom`), the From header address
    must be in that same domain, and the sender domain must be hosted and
    `active` (`GET /api/v1/directory/domains/{domainId}`). mail-in copies
    the blob into the recipient domain before it stores `InboundRaw`, so the
    stored `rawBlobKey` is always under the recipient domain.
  - Errors: `400 invalid_request` (recipients in more than one domain, a
    `rawBlobKey` outside its owner domain, a hand-off From outside the
    sender domain, a size out of range), `503
    unavailable` (storage failed; the edge answers `451` and the sender
    retries).
  - The same `rawBlobKey` sent again answers the same `inboundId`, so a
    retried SMTP transaction is stored once.
- After the message is stored, `mail-in` publishes one `email.received`
  event per recipient domain: envelope `domainId` is that domain, payload
  `events.EmailReceived{inboundId, rcpts[] (only that domain's), rawBlobKey,
  headers: EmailHeaders, authResults: {spf, dkim, dmarc, handoff}}`
  (EVENTS.md section 7); `from`, `to[]` and `cc[]` are address strings, and
  empty lists are `[]`. A retried accept of the same message stages no
  second event (`eventQueued`).
- `mail-in` stages `email.received` with `kit/outbox` in the `inbound-raw`
  document (registered in `outbox-collections`) and drains it on write.
  Outbox routes, all `Allow(Service)`: `POST /api/v1/mail-in/outbox/relay`,
  `GET /api/v1/mail-in/outbox/dead`, `POST /api/v1/mail-in/outbox/dead/requeue`.

## mail-out (worker, no public routes)

Consumes `email.send.requested` (envelope `domainId` is the sender's domain;
payload `{messageId, from, rcpt, role, to[], cc[], subject, inReplyTo,
references[], attachmentIds[], idempotencyKey}`; `to[]` and `cc[]` are the
visible header lists, `rcpt` is the one envelope recipient of this item,
Bcc never appears in `to[]` or `cc[]`), reads the body from `messaging`
(`GET /api/v1/messaging/messages/{messageId}/source`, `Allow(Service)`,
answers `MessageSource{messageId, domainId, body, attachmentIds[]}`;
slice M3), reads `dkimSelector` and the hosted state from `directory`
(`GET /api/v1/directory/domains/{domainId}`, `Allow(Service)`, answers
`Domain`; slice D1), builds MIME, signs DKIM,
delivers, and publishes `email.delivery.changed{messageId, rcpt, status,
detail}`. When `rcpt` is in another hosted domain, `mail-out` may hand the
signed message to `mail-in` inside the install instead of using the
internet. It is still email: full headers, DKIM, and an email conversation
on both sides. The hand-off (slice M1 accepts it, slice M2 sends it):
- mail-out writes the raw message to the blob store under the SENDER's
  domain and calls `POST /api/v1/mail-in/messages` with the same
  `InboundEnvelope` the SMTP edge sends;
- mail-in accepts a `rawBlobKey` in another hosted domain's space only on
  this `Allow(Service)` route, copies the bytes into the recipient domain's
  space, and keeps only its own copy, so each domain's retention and
  deletion stay its own;
- `authResults` carries the explicit marker `handoff: true` and no
  invented SPF, DKIM or DMARC values. mail-in then skips the DNS checks,
  requires that the sender domain is hosted and active (directory) and
  that the `From` domain equals that sender domain, and the UI shows
  "Sent from <domain> on this server" instead of check badges.
