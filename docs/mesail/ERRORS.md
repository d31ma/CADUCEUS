# Mesail error numbers

Every error answer that Mesail code shapes has a Mesail error number in
`ApiError.message`: `[MS<4 digits>]: <plain English message>` (ADR 0009).
`ApiError.code` (for programs) does not change. A number names one failure
case in one app. Numbers are never reused or renumbered; a retired case
keeps its row, marked "retired".

## Blocks

| Numbers | Owner | First number ("unexpected error") |
| --- | --- | --- |
| MS0000-MS0499 | shared `kit` code (for example `kit/oidc`, `kit/outbox`) | MS0000 |
| MS0500-MS0999 | `identity` | MS0500 |
| MS1000-MS1499 | `directory` | MS1000 |
| MS1500-MS1999 | `messaging` | MS1500 |
| MS2000-MS2499 | `channels` | MS2000 |
| MS2500-MS2999 | `mail-in` | MS2500 |
| MS3000-MS3499 | `mail-out` | MS3000 |
| MS3500-MS3999 | `attachments` | MS3500 |
| MS4000-MS4499 | `search` | MS4000 |
| MS4500-MS4999 | `notify` | MS4500 |
| MS5000-MS5499 | `realtime` | MS5000 |
| MS5500-MS5999 | the router (`deploy/router`) | MS5500 |
| MS6000-MS6499 | `devicebox` (ADR 0011, slice G6) | MS6000 |
| MS6500-MS6999 | `edge/calls` (the calls server; ADR 0012, slice G10) | MS6500 |
| MS7000-MS9999 | reserved; architecture assigns a new block | - |

## Rules

- Each slice of Wave 7 (SLICES.md) adds its app's rows here in the same
  commit as the code, sorted by number.
- Numbers inside a block go in order of first use; leave no gaps on
  purpose.
- **One number per case, not per error type** (decided 2026-11-12, from
  E1). When one error type has several cases (for example an upload with
  a bad file name, media type, size or checksum), each case has its own
  number. The error type's file offers one constructor per case (for
  example `InvalidUploadFileName()`), and the code that raises the error
  calls the constructor of its case, so no Service keeps a table from
  fields to numbers. `field` still names the field.
- The catch-all is a `@HandlesError` method that takes plain `error`
  (TACHYON: a concrete error type wins over it): status 500, `code`
  `unavailable`, the block's first number.
- The message text is short, plain English, and never holds a token,
  password, key, file path, address list or message content.
- Each app's tests (built on `kit/apierrors`, slice E0) check that every
  error answer has its exact number and that the number is inside the
  app's block. `ty test` runs app tests in a temporary folder without the
  repository, so it cannot read this file; the gate check of slice E11
  checks this file against the code instead: every number listed once,
  inside its app's block, and every number the code uses listed here.
- Shared kit packages that are copied into one app only (for example
  `kit/channels`) use that app's block, and their error types carry the
  numbers, one constructor per case.
- Answers that TACHYON makes itself (`401` without identity, `403` without
  a role, `422` `@Check` rejections, a malformed body, an error no handler
  takes) have no number until TACHYON request Y1 (ADR 0009).

## Numbers

| Number | App | Status | `code` | Message | Case |
| --- | --- | --- | --- | --- | --- |
| MS0000 | kit | 500 | `unavailable` | Something went wrong. | an unexpected error in shared code |
| MS0500 | identity | 500 | `unavailable` | Something went wrong. | an unexpected error in `identity` |
| MS0501 | identity | 401 | `unauthenticated` | Sign-in failed. Check your details and try again. | a sign-in, session key, password, 2-step or passkey step was refused (one number, so a wrong passkey looks like a wrong password) |
| MS0502 | identity | 401 | `unauthenticated` | Your sign-in has ended. Sign in again. | `GET /me` with a refused access token |
| MS0503 | identity | 429 | `rate_limited` | Too many sign-in attempts. Wait a few minutes. | Keycloak has locked the person for a while |
| MS0504 | identity | 503 | `unavailable` | Sign-in is not available now. Try again later. | Keycloak or the session settings are not usable (also a bad `identity.passkeyorigins`) |
| MS0505 | identity | 403 | `not_a_member` | You are not a member of that domain. | sign-in asked for a domain the person is not in |
| MS0506 | identity | 403 | `no_domain` | This account may not open that domain. | the person has no domain to open, or a non-Operator asked for `*` |
| MS0507 | identity | 403 | `access_revoked` | Your access was removed. Sign in again. | refresh or switch for a disabled or deleted person |
| MS0508 | identity | 400 | `password_policy` | (Keycloak's rule text) | the new password breaks the password policy; `field` `newPassword` |
| MS0509 | identity | 400 | `invalid_code` | The 2-step code is not right. | a wrong 2-step code; `field` `code` |
| MS0510 | identity | 409 | `label_exists` | This passkey name is already used. | rename to a passkey name the person already uses |
| MS0511 | identity | 409 | `too_many` | You have the most passkeys allowed. Remove one first. | passkey setup when the person has the most passkeys |
| MS0512 | identity | 400 | `setup_expired` | The passkey setup has expired. Start again. | add a passkey with an old or broken setup key |
| MS0513 | identity | 400 | `invalid_passkey` | The passkey could not be added. Start again. | Keycloak refused the new passkey |
| MS0514 | identity | 400 | `invalid_request` | A passkey name is 1 to 64 characters. | a passkey name that is empty or too long; `field` `label` |
| MS0515 | identity | 404 | `not_found` | That passkey was not found. | rename or remove a passkey the person does not have |
| MS0516 | identity | 404 | `not_found` | That domain was not found. | switch to a domain the person is not in |
| MS0517 | identity | 400 | `invalid_request` | The domain id must be a domain name or *. | `POST /service-token` with a bad `domainId`; `field` `domainId` |
| MS0518 | identity | 401 | `unauthenticated` | The client id or client secret is not right. | `POST /service-token` with a refused client |
| MS0519 | identity | 503 | `unavailable` | Service tokens are not available now. Try again later. | `POST /service-token` failed for another reason |
| MS0520 | identity | 503 | `unavailable` | The signing keys are not available now. Try again later. | `GET /jwks` without a usable signing key |
| MS1000 | directory | 500 | `unavailable` | Something went wrong. | an unexpected error in `directory` |
| MS1500 | messaging | 500 | `unavailable` | Something went wrong. | an unexpected error in `messaging` |
| MS2000 | channels | 500 | `unavailable` | Something went wrong. | an unexpected error in `channels` |
| MS2001 | channels | 401 | `unauthenticated` | the access token was refused | a channel route could not verify the caller's token |
| MS2002 | channels | 403 | `forbidden` | only members of a domain use its channels | the caller's token has no member or admin role |
| MS2003 | channels | 400 | `invalid_request` | a channel name has 1 to 80 lowercase letters, digits, - or _ | a new channel's name breaks the rule (`field` `name`) |
| MS2004 | channels | 400 | `invalid_request` | visibility is public or private | a new channel's visibility is not `public` or `private` (`field` `visibility`) |
| MS2005 | channels | 400 | `invalid_request` | a topic has at most 250 characters | a channel topic is too long (`field` `topic`) |
| MS2006 | channels | 409 | `conflict` | the channel name ... is already used in this domain | a new channel's name is already used in the domain |
| MS2007 | channels | 404 | `not_found` | no such channel | the channel id in the path does not have the `ch_` form |
| MS2008 | channels | 404 | `not_found` | no such channel | the channel does not exist, or it is private and the caller is not in it |
| MS2009 | channels | 400 | `invalid_request` | invite 1 to 100 members at a time | an invite has no member or more than 100 (`field` `memberIds`) |
| MS2010 | channels | 400 | `invalid_request` | a member id is empty | an invite lists an empty member id (`field` `memberIds`) |
| MS2011 | channels | 404 | `not_found` | no such member in this domain | an invited member is not in the channel's domain |
| MS2012 | channels | 403 | `forbidden` | only a domain admin can set a channel email address | a caller who is not a domain admin set the email address |
| MS2013 | channels | 403 | `forbidden` | only a domain admin or the channel creator can archive a channel | another member tried to archive or unarchive the channel |
| MS2014 | channels | 403 | `forbidden` | join the channel to change its topic | a caller who is not in the channel changed its topic |
| MS2015 | channels | 409 | `conflict` | the channel is archived and read-only | a change, join or invite on an archived channel |
| MS2016 | channels | 400 | `invalid_request` | a channel email address is local-part@ the channel's own domain | the email address has the wrong form (`field` `emailAddress`) |
| MS2017 | channels | 400 | `invalid_request` | the address ... is not in the channel's domain | `directory` refused the email address as outside the domain (`field` `emailAddress`) |
| MS2018 | channels | 409 | `duplicate` | the address ... already belongs to a member, an alias or another channel | the channel email address is already taken |
| MS2019 | channels | 503 | `unavailable` | the channel service is not available | `directory` did not answer a member lookup or an address claim or release |
| MS2020 | channels | 503 | `unavailable` | the channel service is not available | the channel store (FYLO) did not answer |
| MS2021 | channels | 503 | `unavailable` | the channel service is not available | the channel store answered something unreadable |
| MS2022 | channels | 503 | `unavailable` | the channel service is not available / the outbox is not available | the outbox settings are missing |
| MS2023 | channels | 503 | `unavailable` | the outbox is not available | an outbox document cannot be read |
| MS2024 | channels | 404 | `not_found` | (the outbox's own text) | the dead letter to retry is not in the outbox document |
| MS2025 | channels | 404 | `not_found` | no such outbox document | the outbox document to retry does not exist |
| MS2026 | channels | 404 | `not_found` | no such outbox collection | the collection named in a retry is not an outbox collection |
| MS2027 | channels | 503 | `unavailable` | the channel service is not available | any other failure in a channel route |
| MS2028 | channels | 503 | `unavailable` | the outbox is not available | any other failure in an outbox route |
| MS2500 | mail-in | 500 | `unavailable` | Something went wrong. | an unexpected error in `mail-in` |
| MS3000 | mail-out | 500 | `unavailable` | Something went wrong. | an unexpected error in `mail-out` |
| MS3500 | attachments | 500 | `unavailable` | Something went wrong. | an unexpected error in `attachments` |
| MS4000 | search | 500 | `unavailable` | Something went wrong. | an unexpected error in `search` |
| MS4500 | notify | 500 | `unavailable` | Something went wrong. | an unexpected error in `notify` |
| MS5000 | realtime | 500 | `unavailable` | Something went wrong. | an unexpected error in `realtime` |
| MS5001 | realtime | 400 | `invalid_request` | a connection id is required | the gateway connects a connection with no `connectionId` (field `connectionId`) |
| MS5002 | realtime | 400 | `invalid_request` | the kind is socket or stream | the gateway connects a connection whose `kind` is not `socket` or `stream` (field `kind`) |
| MS5003 | realtime | 400 | `invalid_request` | unknown message type | a client message whose `type` is not `auth`, `ticket`, `ping`, `typing` or `presence` (field `type`) |
| MS5004 | realtime | 400 | `invalid_request` | a conversation id is required | a `typing` message with no `conversationId` (field `conversationId`) |
| MS5005 | realtime | 400 | `invalid_request` | must name 1 to 100 members | `GET /presence` with no `memberIds` or more than 100 (field `memberIds`) |
| MS5006 | realtime | 400 | `invalid_request` | must be an RFC 3339 time | the sweep asked for presence due at a time that is not RFC 3339 (field `now`); the sweep route answers it as MS5024 |
| MS5007 | realtime | 400 | `invalid_request` | must be active, idle or dnd | a presence `state` that is not `active`, `idle` or `dnd` (field `state`) |
| MS5008 | realtime | 400 | `invalid_request` | must be an RFC 3339 time in the future | `dnd` with a `doNotDisturbUntil` that is missing or not in the future (field `doNotDisturbUntil`) |
| MS5009 | realtime | 400 | `invalid_request` | Why the envelope is refused, for example "the payload is not a JSON object of the expected shape". | an event envelope refused by the shared `kit/events` (field `envelope`); retired when slice E12 numbers `kit/events` |
| MS5010 | realtime | 400 | `invalid_request` | Why the type is refused, for example "the service realtime does not consume events of type member.upserted". | an event type `realtime` does not consume, or one not in the catalog, refused by the shared `kit/events` (field `type`); retired when slice E12 numbers `kit/events` |
| MS5011 | realtime | 401 | `unauthenticated` | Sign in again. | the bearer token of a presence, tickets, events or sweep request is missing, forged or expired |
| MS5012 | realtime | 401 | `unauthenticated` | Sign in again. | a ticket or socket sign-in with a token that is not a valid member token (for example a service token) |
| MS5013 | realtime | 401 | `unauthenticated` | Sign in again. | a stream sign-in with a ticket that is unknown, already used or expired; the gateway closes the connection with 4401 |
| MS5014 | realtime | 401 | `unauthenticated` | Sign in again. | a `typing` or `presence` message, or a renewal, from a connection that is not signed in; the gateway closes the connection with 4401 |
| MS5015 | realtime | 401 | `unauthenticated` | Sign in again. | a message from a connection whose sign-in has ended; the gateway closes the connection with 4401 |
| MS5016 | realtime | 401 | `unauthenticated` | Sign in again. | a renewal with a token of another person or domain; the gateway closes the connection with 4401 |
| MS5017 | realtime | 403 | `forbidden` | This caller may not use this route. | the sweep caller has no `Service` role |
| MS5018 | realtime | 403 | `forbidden` | This caller may not use this route. | the events caller is not a service (refused by the shared `kit/events`); retired when slice E12 numbers `kit/events` |
| MS5019 | realtime | 404 | `not_found` | Nothing was found. | the event's domain is not the caller's domain |
| MS5020 | realtime | 404 | `not_found` | Nothing was found. | the sweep caller is not install-wide |
| MS5021 | realtime | 404 | `not_found` | Nothing was found. | `GET /presence` for members of whom none has presence |
| MS5022 | realtime | 404 | `not_found` | Nothing was found. | `typing` in a conversation the person is not a member of |
| MS5023 | realtime | 404 | `not_found` | Nothing was found. | a connection that is not known; the gateway routes answer it as MS5014 and close the connection with 4401 |
| MS5024 | realtime | 503 | `unavailable` | Realtime is not available right now. Try again later. | realtime's storage or the connection gateway failed, or a route met an error it does not name |
| MS5500 | router | 502 | `unavailable` | Something went wrong. | an unexpected error in the router |
| MS6000 | devicebox | 500 | `unavailable` | Something went wrong. | an unexpected error in `devicebox` |
| MS6001 | devicebox | 401 | `unauthenticated` | Sign in again. | the token is missing, forged or expired (a token TACHYON lets through but `kit/oidc` refuses) |
| MS6002 | devicebox | 403 | `forbidden` | Only members of this domain can use the mailbox. | a mailbox route called with a token that is neither `Member` nor `DomainAdmin` |
| MS6003 | devicebox | 400 | `invalid_request` | The envelope id must be a UUID. | `envelopeId` is not a UUID (field `envelopeId`) |
| MS6004 | devicebox | 400 | `invalid_request` | The sending device id is not valid. | `senderDeviceId` is empty or not 1 to 128 of `A-Z a-z 0-9 _ -` (field `senderDeviceId`) |
| MS6005 | devicebox | 400 | `invalid_request` | Name 1 to 200 receiving devices. | `toDeviceIds` has 0 or more than 200 ids (field `toDeviceIds`) |
| MS6006 | devicebox | 400 | `invalid_request` | A receiving device id is empty, too long or repeated. | a `toDeviceIds` entry is not a valid device id, or is listed twice (field `toDeviceIds`) |
| MS6007 | devicebox | 400 | `invalid_request` | The kind must be message, group or linking. | `kind` is another value; `undelivered` comes only from the expiry job (field `kind`) |
| MS6008 | devicebox | 400 | `invalid_request` | The ciphertext must be base64url text that is not empty. | `ciphertext` is empty or not base64url (field `ciphertext`) |
| MS6009 | devicebox | 413 | `too_large` | The ciphertext is larger than 256 KiB. | the decoded `ciphertext` is over 262,144 bytes |
| MS6010 | devicebox | 404 | `not_found` | The sending device is not one of your devices in this domain. | `senderDeviceId` is not a device of the caller's person in the token's domain |
| MS6011 | devicebox | 404 | `not_found` | A receiving device is not a device of a member of this domain. | a `toDeviceIds` entry is unknown in the token's domain |
| MS6012 | devicebox | 409 | `duplicate` | An envelope with this id already exists with other content. | a put repeats an `envelopeId` with another sender, devices, kind or ciphertext |
| MS6013 | devicebox | 429 | `box_full` | A receiving device already has 10,000 waiting envelopes. | a receiving device's box count is at 10,000 |
| MS6014 | devicebox | 404 | `not_found` | This device is not one of your devices in this domain. | list or acknowledge for a device of another person or domain, or an id that is not a device id |
| MS6015 | devicebox | 400 | `invalid_request` | The page cursor is not valid. | `after` is not a cursor that `next` gave (field `after`) |
| MS6016 | devicebox | 400 | `invalid_request` | The page size must be 1 to 100. | `limit` is not a number from 1 to 100 (field `limit`) |
| MS6017 | devicebox | 400 | `invalid_request` | Acknowledge 1 to 10 envelopes at a time. | `envelopeIds` has 0 or more than 10 ids (field `envelopeIds`) |
| MS6018 | devicebox | 400 | `invalid_request` | Every acknowledged envelope id must be a UUID. | an `envelopeIds` entry is not a UUID (field `envelopeIds`) |
| MS6019 | devicebox | 503 | `unavailable` | The device list is not available. | `directory`'s `POST /api/v1/directory/devices/owners` failed or answered other than 200 (until G1 ships it, every call) |
| MS6020 | devicebox | 503 | `unavailable` | The device list is not connected yet. | `devicebox.directoryurl` is not set, so the device list cannot be asked |
| MS6021 | devicebox | 503 | `unavailable` | The mailbox store is not available. | a FYLO command or query failed |
| MS6022 | devicebox | 503 | `unavailable` | The mailbox store gave an answer that cannot be read. | a FYLO answer could not be decoded |
| MS6023 | devicebox | 503 | `unavailable` | The outbox settings are missing. | `outbox` is missing from the app settings |
| MS6024 | devicebox | 503 | `unavailable` | Too many events are waiting. Try again later. | `kit/outbox` refused to stage an event (`OutboxFull`) |
| MS6025 | devicebox | 404 | `not_found` | There is no such dead letter. | requeue names an event and subscriber that is not a dead letter |
| MS6026 | devicebox | 404 | `not_found` | There is no such outbox document. | requeue names a document that does not exist |
| MS6027 | devicebox | 404 | `not_found` | There is no such outbox collection. | requeue names a collection that is not in `outbox-collections` |
| MS6028 | devicebox | 503 | `unavailable` | The mailbox is not available. | a mailbox or job route failed in a way no other number covers |
| MS6029 | devicebox | 503 | `unavailable` | The outbox is not available. | an outbox route failed in a way no other number covers |
| MS6030 | devicebox | 503 | `unavailable` | The outbox fields cannot be read. | a document's outbox fields could not be read or written |
| MS6031 | devicebox | 503 | `unavailable` | The device list gave an answer that cannot be read. | `directory`'s owners answer is not JSON or has no `owners` |
