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
- **Keeping constructor lists short** (from E7, 2026-11-14). TACHYON's
  duplicate check (TY1315) flags any block of about 100 tokens over 10
  lines that repeats, and a long list of near-identical constructors
  trips it. Write each case's constructor on one line that reads its
  text from a table of plain literals in the same error type (number,
  code, field, text), and keep the constructors in number order. G1, O2
  and later slices add their cases the same way.
- **What `tools/errornumbers` reads** (type-aware since E15): number
  literals that reach a `Number() int` method; same-named fields and
  methods of other types are ignored.
- **Kit numbers** (slice E12, 2026-11-14). Every error type of a kit
  package copied into more than one app (`kit/events`, `kit/outbox`,
  `kit/ids`, `kit/blobs`, `kit/recipients`, `kit/mime`, `kit/addresses`)
  carries a number from MS0001-MS0499, one constructor per case (or a
  `Number()` method for a type with one case). When an app answers such an
  error as it is, its error handler answers the kit number; it never maps
  it to a number of its own block. When an app turns the kit error into a
  case of its own (for example a missing blob that means "the file is not
  uploaded yet"), it keeps its own number. Kit errors of `kit/apierrors`
  report problems in this file to tests and have no number.
- **`kit/oidc` has no kit numbers** (decided 2026-11-14). Its sign-in,
  passkey, account and session errors are raised only in `identity`, which
  answers them with its own numbers; MS0501 answers every sign-in refusal
  with one number on purpose, so the answer never shows which step failed
  (it stops account probing). Every other app answers a refused token with
  its own "sign in again" number.
- A retired row keeps its number, and its last column starts with
  "retired" and names the number that replaced it. The gate check skips a
  retired row, and refuses code that still uses a retired number.
- Answers that TACHYON makes itself (`401` without identity, `403` without
  a role, `422` `@Check` rejections, a malformed body, an error no handler
  takes) have no number until TACHYON request Y1 (ADR 0009).

## Numbers

| Number | App | Status | `code` | Message | Case |
| --- | --- | --- | --- | --- | --- |
| MS0000 | kit | 500 | `unavailable` | Something went wrong. | an unexpected error in shared code |
| MS0001 | kit | 400 | `invalid_request` | the event envelope is not valid: eventId must not be empty | `kit/events`: an event envelope without `eventId` |
| MS0002 | kit | 400 | `invalid_request` | the event envelope is not valid: domainId is longer than 253 characters | `kit/events`: an envelope `domainId` over 253 characters |
| MS0003 | kit | 400 | `invalid_request` | the event envelope is not valid: domainId may only hold the characters a-z, 0-9, - and . | `kit/events`: an envelope `domainId` with another character |
| MS0004 | kit | 400 | `invalid_request` | the event envelope is not valid: domainId must hold at least one dot | `kit/events`: an envelope `domainId` without a dot |
| MS0005 | kit | 400 | `invalid_request` | the event envelope is not valid: domainId must not hold an empty label | `kit/events`: an envelope `domainId` with an empty label |
| MS0006 | kit | 400 | `invalid_request` | the event type ... is not in the Mesail event catalog | `kit/events`: an event type that is not in the catalog (`UnknownEventType`, also inside an envelope check) |
| MS0007 | kit | 400 | `invalid_request` | the event envelope is not valid: the producer ... does not produce this event type; ... does | `kit/events`: an envelope from a producer the catalog does not list for its type |
| MS0008 | kit | 400 | `invalid_request` | the event envelope is not valid: occurredAt must be an RFC 3339 time | `kit/events`: an envelope `occurredAt` that is not RFC 3339 |
| MS0009 | kit | 400 | `invalid_request` | the event envelope is not valid: payload must not be empty | `kit/events`: an envelope without a payload |
| MS0010 | kit | 400 | `invalid_request` | the event envelope is not valid: payload holds data after its JSON value | `kit/events`: a payload with more data after its JSON value |
| MS0011 | kit | 400 | `invalid_request` | the event envelope is not valid: the payload is not a JSON object of the expected shape | `kit/events`: a payload that does not have the shape its type needs (raised by `realtime`) |
| MS0012 | kit | 403 | `forbidden` | only a caller with the Service role may deliver events | `kit/events`: an events route called without the `Service` role (`realtime` answers its fixed forbidden text) |
| MS0013 | kit | 400 | `invalid_request` | the service ... does not consume events of type ... | `kit/events`: an event type the receiving service does not consume |
| MS0014 | kit | 404 | `not_found` | the document ... was not found | `kit/outbox`: an outbox document that does not exist (the memory store, and mail-out's outbound items) |
| MS0015 | kit | 404 | `not_found` | the dead letter ... for ... was not found | `kit/outbox`: a requeue names an event and subscriber that is not a dead letter |
| MS0016 | kit | 503 | `unavailable` | the application JSON has no "outbox" settings | `kit/outbox`: `outbox` is missing from the app settings (apps answer their own settings number, which also covers a file that cannot be read) |
| MS0017 | kit | 503 | `unavailable` | the document's waiting events would take ... bytes, over the ... byte document cap; try again later | `kit/outbox`: staging an event would pass the document cap (`OutboxFull`; `devicebox` answers its own text) |
| MS0018 | kit | - | - | delivery to ... failed (status ..., permanent ...): ... | `kit/outbox`: a push to a subscriber failed; kept as a dead letter's `lastError`, never an answer |
| MS0019 | kit | - | - | expected exactly one staged ... event, found ... | `kit/outbox`: the test helper `Published` found no single staged event; never an answer |
| MS0020 | kit | 404 | `not_found` | the id does not start with "..." | `kit/ids`: a wire id with another prefix (apps turn it into their own not found) |
| MS0021 | kit | 404 | `not_found` | the blob key domain "..." is not allowed | `kit/blobs`: a blob key with a bad domain |
| MS0022 | kit | 404 | `not_found` | the blob key name "..." is not allowed | `kit/blobs`: a blob key with a bad name |
| MS0023 | kit | 404 | `not_found` | no blob is stored at "..." | `kit/blobs`: no blob under the key (apps turn it into their own case) |
| MS0024 | kit | 413 | `too_large` | the blob is larger than N bytes | `kit/blobs`: a blob or upload over the size limit; `attachments` answers it with `field` `size` |
| MS0025 | kit | 400 | `invalid_request` | expected N bytes but received M | `kit/blobs`: a stored blob has another size than promised |
| MS0026 | kit | 403 | `forbidden` | the blob link signature is not valid | `kit/blobs`: a signed blob link with a bad signature |
| MS0027 | kit | 403 | `forbidden` | the blob link has expired | `kit/blobs`: a signed blob link past its expiry |
| MS0028 | kit | 502 | `unavailable` | the S3 ... request answered HTTP ... | retired (OP6 removed the S3 store, so `kit/blobs` has no `S3Failure`); was: `kit/blobs`: the S3 store answered an unexpected status |
| MS0029 | kit | 503 | `unavailable` | the S3 blob store needs an endpoint setting ... | retired (OP6 removed the S3 store, so `kit/blobs` has no `MissingEndpoint`); was: `kit/blobs`: the S3 store has no endpoint setting (at start-up) |
| MS0030 | kit | 400 | `invalid_request` | "..." is not a valid domain name | `kit/recipients`: a recipient domain name that is not valid |
| MS0031 | kit | 400 | `invalid_request` | "..." is not a valid email address | `kit/recipients`: a recipient address that is not valid |
| MS0032 | kit | 422 | `invalid_request` | delivery "..." is not allowed for a ... recipient | `kit/recipients`: a delivery the recipient kind does not allow |
| MS0033 | kit | 400 | `invalid_request` | the message header block cannot be read | `kit/mime`: a message whose header block cannot be read |
| MS0034 | kit | 413 | `too_large` | the message or its header block is larger than the size limit | `kit/mime`: a message or header block over its limit |
| MS0035 | kit | 400 | `invalid_request` | the message has more nested parts than the depth limit | `kit/mime`: a message nested deeper than the limit |
| MS0036 | kit | 400 | `invalid_request` | the message has more parts than the part limit | `kit/mime`: a message with more parts than the limit |
| MS0037 | kit | 400 | `invalid_request` | ... must hold only ASCII characters | `kit/addresses`: a name with a character that is not ASCII |
| MS0038 | kit | 400 | `invalid_request` | address must hold an @ between the local part and the domain | `kit/addresses`: an address without `@` |
| MS0039 | kit | 400 | `invalid_request` | domain must be at most 253 characters | `kit/addresses`: a domain over 253 characters |
| MS0040 | kit | 400 | `invalid_request` | domain must be labels of 1 to 63 characters ... | `kit/addresses`: a domain with a bad label |
| MS0041 | kit | 400 | `invalid_request` | ... must be 1 to 64 characters | `kit/addresses`: a local part or username that is empty or over 64 characters |
| MS0042 | kit | 400 | `invalid_request` | ... may only hold a-z, 0-9, ., _ and - | `kit/addresses`: a local part or username with another character |
| MS0043 | kit | 400 | `invalid_request` | ... must start and end with a letter or digit | `kit/addresses`: a local part or username that starts or ends with `.`, `_` or `-` |
| MS0044 | kit | 400 | `invalid_request` | ... must not hold two dots in a row | `kit/addresses`: a local part or username with `..` |
| MS0045 | kit | 400 | `invalid_request` | username must not be a role name | `kit/addresses`: a username that is a role name |
| MS0046 | kit | 409 | `conflict` | the local part ... is already in use | `kit/addresses`: a local part another member, alias or channel holds |
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
| MS1001 | directory | 401 | `unauthenticated` | the access token was refused | a directory route could not verify the caller's token |
| MS1002 | directory | 403 | `forbidden` | only an Operator may do this | a domains or people route that only an Operator may call |
| MS1003 | directory | 404 | `not_found` | not found | a service route for another domain, or an install-wide route called with a domain token |
| MS1004 | directory | 403 | `forbidden` | you are not a member of this domain | the caller is not an active member of the token's domain |
| MS1005 | directory | 403 | `forbidden` | this domain is not hosted here | the token's domain is missing, disabled or being removed |
| MS1006 | directory | 403 | `forbidden` | only an Operator or an admin of every domain of this person may change the account | an account change by someone who does not manage every domain of the person |
| MS1007 | directory | 400 | `invalid_request` | limit must be between 1 and 100 | a page `limit` outside 1 to 100; `field` `limit` |
| MS1008 | directory | 400 | `invalid_request` | limit must be a whole number | a page `limit` that is not a number; `field` `limit` |
| MS1009 | directory | 400 | `invalid_request` | the cursor is not one this service gave out | a page `cursor` this service did not give out; `field` `cursor` |
| MS1010 | directory | 404 | `not_found` | no hosted domain ... | a domain id that is not hosted |
| MS1011 | directory | 400 | `invalid_request` | (the domain name rule) | retired (E12): now MS0030; was: a new domain name that is not a valid domain name; `field` `name` |
| MS1012 | directory | 409 | `duplicate` | ... is already hosted | add a domain that is already hosted |
| MS1013 | directory | 503 | `unavailable` | the DNS lookup failed | the DNS check of a domain failed |
| MS1014 | directory | 404 | `domain_not_hosted` | this domain is not hosted here | an address lookup for a domain that is not active here |
| MS1015 | directory | 404 | `not_found` | no such address | an address lookup for an address nobody owns |
| MS1016 | directory | 400 | `invalid_request` | the address must be in your own domain | an alias or channel address outside the caller's domain; `field` `address` |
| MS1017 | directory | 404 | `not_found` | no such member in your domain | a member id that is not an active member of the caller's domain |
| MS1018 | directory | 409 | `duplicate` | ... is already in use | an alias or channel address that another owner uses |
| MS1019 | directory | 400 | `invalid_request` | (the name rule) | a username or address name check failed for a reason that is not a `kit/addresses` name rule; `field` `username` or `address` (the name rules answer MS0037-MS0045 since E12) |
| MS1020 | directory | 409 | `duplicate` | (the reservation text) | retired (E12): now MS0046; was: a username or address name reserved by someone else |
| MS1021 | directory | 404 | `not_found` | no such channel address | remove a channel address that does not exist or is not a channel's |
| MS1022 | directory | 400 | `invalid_request` | a channel id is required | a channel address without a channel id; `field` `channelId` |
| MS1023 | directory | 400 | `invalid_request` | at most 100 addresses can be resolved at once | resolve more than 100 addresses; `field` `addresses` |
| MS1024 | directory | 400 | `invalid_request` | (the address rule) | retired (E12): now MS0030 or MS0031; was: resolve an address that is not valid; `field` `addresses` |
| MS1025 | directory | 409 | `duplicate` | ... is used by an alias or a channel address | add a member whose username an alias or channel address uses |
| MS1026 | directory | 400 | `invalid_request` | a display name is required for a new person | add a new person without a display name; `field` `displayName` |
| MS1027 | directory | 400 | `invalid_request` | a first password is required for a new person | add a new person without a first password; `field` `firstPassword` |
| MS1028 | directory | 409 | `duplicate` | ... belonged to a deleted person and is never reused | add a member with the username of a deleted person |
| MS1029 | directory | 409 | `duplicate` | ... is already a member of ... | add a person to a domain they are already in |
| MS1030 | directory | 409 | `duplicate` | this username is already taken | Keycloak already has an account with this username |
| MS1031 | directory | 400 | `password_policy` | (Keycloak's rule text) | Keycloak refused the password; `field` `password` |
| MS1032 | directory | 404 | `not_found` | no such account | Keycloak has no account for the person |
| MS1033 | directory | 503 | `unavailable` | the identity server is not available | Keycloak failed or did not answer |
| MS1034 | directory | 409 | `last_admin` | a domain must keep at least one admin | a change or removal would leave the domain without an admin |
| MS1035 | directory | 400 | `invalid_request` | state must be active or disabled | a person change with another state; `field` `state` |
| MS1036 | directory | 400 | `invalid_request` | a new password is required | a password reset without a password; `field` `newPassword` |
| MS1037 | directory | 404 | `not_found` | you are not a member of ... | set a default domain the caller is not a member of |
| MS1038 | directory | 404 | `not_found` | no such person | a person id that does not exist |
| MS1039 | directory | 404 | `not_found` | this person was deleted | a person id of a deleted person |
| MS1040 | directory | 404 | `not_found` | not found | the development seed route when the seed is turned off |
| MS1041 | directory | 400 | `invalid_request` | a person id is required | a seed without a person id; `field` `personId` |
| MS1042 | directory | 400 | `invalid_request` | a display name is required | a seed without a display name; `field` `displayName` |
| MS1043 | directory | 400 | `invalid_request` | a seed call adds at most 2 domains; seed the person again for more | a seed with more than 2 domains; `field` `memberships` |
| MS1044 | directory | 503 | `unavailable` | the outbox is not configured | the outbox settings cannot be read |
| MS1045 | directory | 404 | `not_found` | no such dead event | requeue a dead event in a document whose outbox fields cannot be read (a missing dead event answers MS0015 since E12) |
| MS1046 | directory | 404 | `not_found` | no such outbox collection | the outbox store names a collection it does not know |
| MS1047 | directory | 404 | `not_found` | no such outbox document | the outbox store cannot find its document |
| MS1048 | directory | 503 | `unavailable` | the directory store is not available | the directory store (FYLO) failed |
| MS1049 | directory | 400 | `invalid_request` | deviceId must be a UUID | add a device whose id is not a lower-case UUID (field `deviceId`) |
| MS1050 | directory | 400 | `invalid_request` | name must be 1 to 64 characters | add a device with an empty or too long name (field `name`) |
| MS1051 | directory | 400 | `invalid_request` | identityKey must be a base64url Ed25519 public key of 32 bytes | add a device with a bad identity key (field `identityKey`) |
| MS1052 | directory | 400 | `invalid_request` | agreementKey must be a base64url X25519 public key of 32 bytes | add a device with a bad agreement key (field `agreementKey`) |
| MS1053 | directory | 400 | `invalid_request` | backupKey must be a base64url Ed25519 public key of 32 bytes | add a device with a bad backup key (field `backupKey`) |
| MS1054 | directory | 400 | `invalid_request` | personIds must name 1 to 50 people | list devices of no person or of more than 50 people (field `personIds`) |
| MS1055 | directory | 400 | `invalid_request` | deviceIds must name 1 to 200 devices | ask for the owners of no device or of more than 200 devices (field `deviceIds`) |
| MS1056 | directory | 403 | `forbidden` | a new device must be signed by one of your devices or by your backup key | add a later device with no signature, a wrong one, or a signer that is not one of the person's devices or backup key |
| MS1057 | directory | 409 | `duplicate` | this device id is already in use | add a device whose id is already listed (for anyone) |
| MS1058 | directory | 404 | `not_found` | no such device | remove a device that does not exist or belongs to another person |
| MS1059 | directory | 400 | `invalid_request` | a device added with the backup key cannot change the backup key | add a device signed by `backup` that also sends `backupKey` (field `backupKey`) |
| MS1500 | messaging | 500 | `unavailable` | Something went wrong. | an unexpected error in `messaging` |
| MS1501 | messaging | 401 | `unauthenticated` | the access token was refused | a messaging route could not verify the caller's token |
| MS1502 | messaging | 403 | `forbidden` | only members of a domain use its conversations | a chat route called without the Member or DomainAdmin role |
| MS1503 | messaging | 403 | `forbidden` | only a caller with the Service role may use this route | `POST /events` or `GET /messages/{id}/source` without the Service role |
| MS1504 | messaging | 403 | `forbidden` | the outbox is managed with an install-wide token only | an outbox route called with a domain token |
| MS1505 | messaging | 404 | `not_found` | no such domain | a service token of one domain delivered an event of another domain |
| MS1506 | messaging | 400 | `invalid_request` | limit must be a whole number from 1 to 100 | a page `limit` that is not a whole number from 1 to 100; `field` `limit` |
| MS1507 | messaging | 503 | `unavailable` | the messaging service is not available | a route failed with an error no other case names (also a full outbox document) |
| MS1508 | messaging | 400 | `invalid_request` | (the event check's text) | retired (E12): now MS0001-MS0011; was: `POST /events` with an envelope that is not valid (`kit/events`) |
| MS1509 | messaging | 400 | `invalid_request` | (the event check's text) | retired (E12): now MS0013; was: `POST /events` with an event type messaging does not consume (`kit/events`) |
| MS1510 | messaging | 400 | `invalid_request` | (the event check's text) | retired (E12): now MS0006; was: `POST /events` with an event type that is not in the catalog (`kit/events`) |
| MS1511 | messaging | 400 | `invalid_request` | messaging does not handle the event type ... | `POST /events` with a consumed type that has no handler |
| MS1512 | messaging | 403 | `forbidden` | only a caller with the Service role may deliver events | retired (E12): now MS0012; was: the event receiver refused a caller without the Service role (`kit/events`) |
| MS1513 | messaging | 400 | `invalid_request` | limit must be between 1 and 100 | the reader got a page size outside 1 to 100; `field` `limit` |
| MS1514 | messaging | 400 | `invalid_request` | the cursor is not valid | a page `cursor` that messaging did not make; `field` `cursor` |
| MS1515 | messaging | 400 | `invalid_request` | the reaction is not a known reaction name | a reaction name outside the list; `field` `reaction` |
| MS1516 | messaging | 400 | `invalid_request` | the body must have 1 to 40000 characters | an edit with an empty or too long body; `field` `body` |
| MS1517 | messaging | 400 | `invalid_request` | recipients can be added only to a direct conversation | a reply in a channel that names recipients; `field` `recipients` |
| MS1518 | messaging | 400 | `invalid_request` | no such attachment: ... | a send names an attachment the domain does not have; `field` `attachmentIds` |
| MS1519 | messaging | 400 | `invalid_request` | the attachment is not ready: ... | a send names an attachment that is not uploaded yet; `field` `attachmentIds` |
| MS1520 | messaging | 400 | `invalid_request` | a direct conversation has at most 100 member participants | a send would give a direct conversation a 101st member (K5); `field` `recipients` |
| MS1521 | messaging | 400 | `invalid_request` | an email needs a subject | an email send without a subject; `field` `subject` |
| MS1522 | messaging | 400 | `invalid_request` | this message needs too many writes; send it to fewer new recipients | a send over the write budget of one request; `field` `recipients` |
| MS1523 | messaging | 400 | `invalid_request` | clientMessageId must have 1 to 200 characters | `field` `clientMessageId` |
| MS1524 | messaging | 400 | `invalid_request` | the body may have at most 40000 characters | a send with a too long body; `field` `body` |
| MS1525 | messaging | 400 | `invalid_request` | the message needs a body or an attachment | a send with neither; `field` `body` |
| MS1526 | messaging | 400 | `invalid_request` | a message may have at most 10 attachments | `field` `attachmentIds` |
| MS1527 | messaging | 400 | `invalid_request` | an attachment id must start with att_ | `field` `attachmentIds` |
| MS1528 | messaging | 400 | `invalid_request` | a message may have at most 20 recipients | `field` `recipients` |
| MS1529 | messaging | 400 | `invalid_request` | each recipient needs an address, a role, a delivery and an expected kind | `field` `recipients` |
| MS1530 | messaging | 400 | `invalid_request` | a Bcc recipient gets email only | a Bcc recipient asked for chat; `field` `recipients` |
| MS1531 | messaging | 400 | `invalid_request` | each address may appear only once | `field` `recipients` |
| MS1532 | messaging | 400 | `invalid_request` | a new conversation needs recipients | a send with no conversation and no recipients; `field` `recipients` |
| MS1533 | messaging | 400 | `invalid_request` | alsoToConversation is allowed only with threadRootId | `field` `alsoToConversation` |
| MS1534 | messaging | 400 | `invalid_request` | the From address must be one of your own addresses | `field` `fromAddress` |
| MS1535 | messaging | 400 | `invalid_request` | a channel gets chat only: ... | a channel recipient asked for email (one channel address); `field` `recipients` |
| MS1536 | messaging | 400 | `invalid_request` | a channel must be the only recipient of a new message, in To | `field` `recipients` |
| MS1537 | messaging | 403 | `forbidden` | only the author can change this message | an edit or delete by someone else |
| MS1538 | messaging | 404 | `not_found` | no such conversation | the conversation does not exist or the caller is not in it |
| MS1539 | messaging | 404 | `not_found` | no such message | the message does not exist or the caller cannot read it |
| MS1540 | messaging | 404 | `not_found` | no such attachment | attachment access for an attachment the caller cannot read |
| MS1541 | messaging | 404 | `not_found` | no such inbound message | `GET /messages/{id}/original` when mail-in has no original |
| MS1542 | messaging | 404 | `not_found` | no such outbox document | a requeue names a document that does not exist |
| MS1543 | messaging | 404 | `not_found` | no such outbox collection | a requeue names a collection that is not messaging's |
| MS1544 | messaging | 404 | `not_found` | no such dead event | a requeue whose outbox document cannot be read (a missing dead letter answers MS0015 since E12) |
| MS1545 | messaging | 409 | `message_deleted` | the message is deleted | an edit, or a reaction added or removed, on a deleted message |
| MS1546 | messaging | 409 | `conflict` | the channel is archived and read-only | a send or change in an archived channel |
| MS1547 | messaging | 409 | `recipients_changed` | the recipients changed | how a recipient gets the message changed since the client asked; the addresses are in `addresses` |
| MS1548 | messaging | 413 | `too_large` | the message is too large to store; send a shorter body | the stored message would pass its size budget |
| MS1549 | messaging | 413 | `too_large` | the attachments of a message may have at most 50 MiB together | a send whose attachments pass 50 MiB together |
| MS1550 | messaging | 422 | `chat_not_allowed` | chat is not allowed for ... | a chat recipient that cannot get chat (one address); `field` the address |
| MS1551 | messaging | 503 | `unavailable` | the message store is not available | FYLO failed |
| MS1552 | messaging | 503 | `unavailable` | the directory is not available | `directory` failed during a send |
| MS1553 | messaging | 503 | `unavailable` | the attachments service is not available | `attachments` failed during a send |
| MS1554 | messaging | 503 | `unavailable` | the text renderer is not available | the rich text renderer failed |
| MS1555 | messaging | 503 | `unavailable` | the messaging storage is not available | FYLO failed while an event was handled |
| MS1556 | messaging | 503 | `unavailable` | ... is not stored yet | an event names a conversation or message messaging has not stored yet (the sender retries) |
| MS1557 | messaging | 503 | `unavailable` | no service token for the domain | messaging could not get a service token to call another service |
| MS1558 | messaging | 503 | `unavailable` | the service answered ... | another service answered with an unexpected status |
| MS1559 | messaging | 503 | `unavailable` | the service is not available | another service could not be reached |
| MS1560 | messaging | 503 | `unavailable` | the directory does not know the resolve route | `directory` answered 404 for its resolve route |
| MS1561 | messaging | 503 | `unavailable` | the outbox relay route answered ... | the scheduled relay call got an unexpected status |
| MS1562 | messaging | 503 | `unavailable` | the outbox settings are missing | the outbox settings in `application.json` cannot be read |
| MS1563 | messaging | 409 | `edit_conflict` | the message changed since you read it | an edit whose `ifVersion` is not the message's `version`; the current message is in `current` |
| MS1564 | messaging | 400 | `invalid_request` | a sync takes 1 to 50 cursors | `POST /sync` with no cursor or more than 50; `field` `cursors` |
| MS1565 | messaging | 400 | `invalid_request` | writtenAt must be a date and time like 2026-11-14T08:12:00Z | a send whose `writtenAt` is not an RFC 3339 time; `field` `writtenAt` |
| MS1566 | messaging | 403 | `forbidden` | scheduled jobs run with an install-wide token only | `POST /jobs/changes-cleanup` with a domain token |
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
| MS2024 | channels | 404 | `not_found` | (the outbox's own text) | retired (E12): now MS0015; was: the dead letter to retry is not in the outbox document |
| MS2025 | channels | 404 | `not_found` | no such outbox document | the outbox document to retry does not exist |
| MS2026 | channels | 404 | `not_found` | no such outbox collection | the collection named in a retry is not an outbox collection |
| MS2027 | channels | 503 | `unavailable` | the channel service is not available | any other failure in a channel route |
| MS2028 | channels | 503 | `unavailable` | the outbox is not available | any other failure in an outbox route |
| MS2500 | mail-in | 500 | `unavailable` | Something went wrong. | an unexpected error in `mail-in` |
| MS2501 | mail-in | 400 | `invalid_request` | between 1 and 100 recipients are required | an inbound envelope has no recipient or more than 100 (`field` `recipients`) |
| MS2502 | mail-in | 400 | `invalid_request` | all recipients must be in one domain | the recipients of an inbound envelope are in more than one domain (`field` `recipients`) |
| MS2503 | mail-in | 400 | `invalid_request` | the recipient domain is not valid | the recipient domain has no dot or holds `/`, `\`, a space or `@` (`field` `recipients`) |
| MS2504 | mail-in | 400 | `invalid_request` | a hand-off must come from an address of the sender domain | a hand-off whose From address is not in the sender domain (`field` `headers.from`) |
| MS2505 | mail-in | 400 | `invalid_request` | the raw blob key must be a blob of the domain that owns it | the raw blob key is not a blob of the owning domain (`field` `rawBlobKey`) |
| MS2506 | mail-in | 400 | `invalid_request` | the size must be between 1 byte and 64 MiB | the message size is 0 or over 64 MiB (`field` `size`) |
| MS2507 | mail-in | 400 | `invalid_request` | the sender domain is not hosted or not active | a hand-off from a domain that is not hosted or not active (`field` `envelopeFrom`) |
| MS2508 | mail-in | 400 | `invalid_request` | no raw message is stored under this key | a hand-off whose raw message is not in the blob store (`field` `rawBlobKey`) |
| MS2509 | mail-in | 401 | `unauthenticated` | The access token was refused. | the caller's token for the original or the content of a message is not valid |
| MS2510 | mail-in | 404 | `not_found` | no such outbox collection | an outbox route names a collection mail-in does not have |
| MS2511 | mail-in | 404 | `not_found` | no such inbound message | the inbound message does not exist or is in another domain |
| MS2512 | mail-in | 404 | `not_found` | no such dead letter | retired (E12): now MS0015; was: a requeue names a dead letter that does not exist |
| MS2513 | mail-in | 404 | `not_found` | no such outbox document | a requeue names an outbox document that does not exist |
| MS2514 | mail-in | 503 | `unavailable` | Inbound storage is not available. Try again later. | the inbound store, the blob store or the directory failed, or a blob setting still names the removed S3 adapter or a bucket (slice OP6; the log names the setting) |
| MS3000 | mail-out | 500 | `unavailable` | Something went wrong. | an unexpected error in `mail-out` |
| MS3001 | mail-out | 400 | `invalid_request` | Why the event is refused, for example "the event envelope is not valid: eventId must not be empty". | retired (E12): now MS0001-MS0011 and MS0013; was: a refused event by a rule of the shared `kit/events` (bad envelope, wrong producer, a type mail-out does not consume) |
| MS3002 | mail-out | 403 | `forbidden` | Only a caller with the Service role may deliver events. | retired (E12): now MS0012; was: the caller of the events route is not a service |
| MS3003 | mail-out | 404 | `not_found` | Not found. | the event's domain is not the caller's domain |
| MS3004 | mail-out | 400 | `invalid_request` | the event envelope is not valid: <the JSON error> | the events body is not a JSON event envelope |
| MS3005 | mail-out | 400 | `invalid_request` | the email.send.requested field producer is missing or not valid | `email.send.requested` from a producer mail-out does not read messages from |
| MS3006 | mail-out | 400 | `invalid_request` | the email.send.requested field payload is missing or not valid | the `email.send.requested` payload cannot be read |
| MS3007 | mail-out | 400 | `invalid_request` | the email.send.requested field messageId is missing or not valid | the `email.send.requested` payload has no `messageId` |
| MS3008 | mail-out | 400 | `invalid_request` | the email.send.requested field idempotencyKey is missing or not valid | the `email.send.requested` payload has no `idempotencyKey` |
| MS3009 | mail-out | 400 | `invalid_request` | the email.send.requested field rcpt is missing or not valid | the `email.send.requested` payload has no valid `rcpt` address |
| MS3010 | mail-out | 400 | `invalid_request` | the email.send.requested field from is missing or not valid | the `from` address of `email.send.requested` is not in the event's domain |
| MS3500 | attachments | 500 | `unavailable` | Something went wrong. | an unexpected error in `attachments` |
| MS3501 | attachments | 400 | `invalid_request` | Give a file name of 1 to 255 bytes without slashes or control characters. | an upload with a bad file name; `field` is `fileName` |
| MS3502 | attachments | 400 | `invalid_request` | Give a media type such as image/png. | an upload with a bad media type; `field` is `mediaType` |
| MS3503 | attachments | 400 | `invalid_request` | An attachment must have at least one byte. | an upload with a size below one byte; `field` is `size` |
| MS3504 | attachments | 400 | `invalid_request` | Give the SHA-256 of the file as 64 lowercase hex characters. | an upload with a bad checksum; `field` is `sha256` |
| MS3505 | attachments | 401 | `unauthenticated` | Sign in again. | the access token was refused |
| MS3506 | attachments | 403 | `forbidden` | Only a service may read attachment details. | a caller that is not a service asks for attachment details |
| MS3507 | attachments | 404 | `not_found` | The attachment was not found. | no such attachment in the caller's domain, or the caller may not see it |
| MS3508 | attachments | 409 | `conflict` | The file has not been uploaded yet. | complete before the file was uploaded; `field` is `file` |
| MS3509 | attachments | 409 | `conflict` | The uploaded file is smaller than the size you gave. | the uploaded file has another size; `field` is `size` |
| MS3510 | attachments | 409 | `conflict` | The uploaded file does not match the SHA-256 you gave. | the uploaded file has another checksum; `field` is `sha256` |
| MS3511 | attachments | 409 | `conflict` | The attachment is not ready to download. | a download before the upload is complete; `field` is `state` |
| MS3512 | attachments | 413 | `too_large` | the file is larger than N bytes | retired (E12): now MS0024; was: an upload larger than the limit; `field` is `size` |
| MS3513 | attachments | 503 | `unavailable` | Attachment storage is not available. Try again later. | the attachment store or link signing failed, or a setting still names the removed S3 adapter or a bucket (slice OP6; the log names the setting) |
| MS3514 | attachments | 404 | `not_found` | No backup was found. | the caller has no completed chat backup in the token's domain (G4) |
| MS3515 | attachments | 400 | `invalid_request` | Give the backup version you saw last, 0 or more. | a backup start with a negative `expectedVersion`; `field` is `expectedVersion` |
| MS3516 | attachments | 400 | `invalid_request` | A backup must have at least one byte. | a backup start with a size below one byte; `field` is `size` |
| MS3517 | attachments | 400 | `invalid_request` | Give 1 to 10 key slots of at most 1 KiB each, of kind recovery or passkey, with base64url values. | a backup start with missing, too many or malformed key slots (a `recovery` slot needs 600,000 rounds or more and no `credentialId`; a `passkey` slot needs a `credentialId`); `field` is `keySlots` |
| MS3518 | attachments | 400 | `invalid_request` | Give the id of the device that made the backup: 1 to 128 letters, digits, - or _. | a backup start with a bad `deviceId`; `field` is `deviceId` |
| MS3519 | attachments | 409 | `conflict` | Another device replaced the backup. Download it, merge, and try again. | a backup start whose `expectedVersion` is not the current version, or a complete for a version that is no longer the newest; `field` is `expectedVersion` |
| MS3520 | attachments | 409 | `conflict` | No upload of this backup version is waiting, or its link has expired. Start the backup again. | a backup complete with no started upload of that version, or after the upload link's life plus one more link life; `field` is `file` |
| MS3521 | attachments | 403 | `forbidden` | Only a service may send events. | a caller without the Service role posts to `/api/v1/attachments/events` |
| MS3522 | attachments | 403 | `forbidden` | A service may send events only for its own domain. | a domain service token posts an event of another domain |
| MS3523 | attachments | 400 | `invalid_request` | The event is not valid. | a malformed envelope or payload, or an event type `attachments` does not take; `field` is `envelope` |
| MS4000 | search | 500 | `unavailable` | Something went wrong. | an unexpected error in `search` |
| MS4001 | search | 400 | `invalid_request` | the search text is too long | a search with too long a text; `field` is `q` |
| MS4002 | search | 400 | `invalid_request` | from must be a date or an RFC 3339 time | a search with a bad date filter; `field` is `from` or `to` |
| MS4003 | search | 400 | `invalid_request` | isEmail must be true or false | a search with a bad true-or-false filter; `field` is `hasAttachment` or `isEmail` |
| MS4004 | search | 400 | `invalid_request` | (the cursor problem) | a search with a bad cursor; `field` is `cursor` |
| MS4005 | search | 400 | `invalid_request` | limit must be a number from 1 to 100 | a search with a bad limit; `field` is `limit` |
| MS4006 | search | 400 | `invalid_request` | createdAt is not an RFC 3339 time | an event with a bad time; `field` is `createdAt` or `editedAt` |
| MS4007 | search | 400 | `invalid_request` | messageId and conversationId are required | an event without message ids; `field` is `payload` |
| MS4008 | search | 400 | `invalid_request` | the event payload does not match its type | an event whose payload does not match its type; `field` is `payload` |
| MS4009 | search | 400 | `invalid_request` | the value cannot be written as JSON | a value that cannot be written as JSON; `field` is `body` |
| MS4010 | search | 400 | `invalid_request` | the value does not have the expected fields | a body without the expected fields; `field` is `body` |
| MS4011 | search | 400 | `invalid_request` | (the envelope problem) | retired (E12): now MS0001-MS0011 and MS0013; was: an event with a bad envelope, or of a type `search` does not take; `field` is `envelope` |
| MS4012 | search | 401 | `unauthenticated` | Sign in again. | the access token was refused |
| MS4013 | search | 403 | `forbidden` | only members of a domain search its messages | a caller that is not a member searches |
| MS4014 | search | 403 | `forbidden` | (the reason) | retired (E12): now MS0012; was: an event from a caller that is not a service |
| MS4015 | search | 503 | `unavailable` | Search is not available. Try again later. | the search index or the membership check failed |
| MS4500 | notify | 500 | `unavailable` | Something went wrong. | an unexpected error in `notify` |
| MS4501 | notify | 401 | `unauthenticated` | Sign in again. | a notify route could not verify the caller's token |
| MS4502 | notify | 404 | `not_found` | The install-wide route was not found. | an operator-only route (jobs, outbox) called with a token that is not install-wide |
| MS4503 | notify | 403 | `forbidden` | Only another Mesail service may call this route. | `POST /call-pushes` with a token that has no `Service` role (`POST /events` answers MS0012 since E12) |
| MS4504 | notify | 404 | `not_found` | The domain was not found. | an event for another domain than the calling service's own, or `POST /call-pushes` with an install-wide token |
| MS4505 | notify | 400 | `invalid_request` | (the event check's text) | an event payload notify cannot read, or another event failure no other number names (`field` `envelope`; `kit/events` refusals answer their kit numbers since E12) |
| MS4506 | notify | 400 | `invalid_request` | notify does not handle events of type ... | an event type notify does not handle (`field` `type`) |
| MS4507 | notify | 400 | `invalid_request` | the time is not an RFC 3339 time | a time in an event payload is not RFC 3339 (`field` `time`) |
| MS4508 | notify | 400 | `invalid_request` | limit must be a whole number from 1 to 100 | `GET /notifications` with a bad `limit` (`field` `limit`) |
| MS4509 | notify | 400 | `invalid_request` | cursor is not readable | `GET /notifications` with a bad `cursor` (`field` `cursor`) |
| MS4510 | notify | 404 | `not_found` | The notification was not found. | mark read for a notification id that is unknown, has another prefix, or belongs to someone else |
| MS4511 | notify | 404 | `not_found` | The message was not found. | `GET /messages/{id}/source` for a summary that is unknown or in another domain |
| MS4512 | notify | 400 | `invalid_request` | must be 0 or between 15 and 1440 | `PUT /settings` with a bad summary delay (`field` `emailSummaryAfterMinutes`) |
| MS4513 | notify | 400 | `invalid_request` | (the settings check's text) | `PUT /settings` with too many or bad muted conversation ids (`field` `mutedConversationIds`) |
| MS4514 | notify | 400 | `invalid_request` | (the endpoint check's text) | a push subscription endpoint that is not an allowed https URL (`field` `endpoint`) |
| MS4515 | notify | 400 | `invalid_request` | the subscription keys p256dh and auth are required | a push subscription without its keys, or with keys too long (`field` `p256dh`) |
| MS4516 | notify | 503 | `unavailable` | Web push is not configured on this server. | web push is turned off or its keys are missing |
| MS4517 | notify | 503 | `unavailable` | Notification storage is not available. Try again later. | the notify store (FYLO) did not answer |
| MS4518 | notify | 503 | `unavailable` | Notification storage is not available. Try again later. | the outbox settings cannot be read |
| MS4519 | notify | 503 | `unavailable` | Notification storage is not available. Try again later. | the outbox relay failed for another reason |
| MS4520 | notify | 404 | `not_found` | The outbox document was not found. | the outbox document to retry does not exist |
| MS4521 | notify | 404 | `not_found` | The outbox collection was not found. | the collection named in a retry is not an outbox collection |
| MS4522 | notify | 404 | `not_found` | The dead letter was not found. | retired (E12): now MS0015; was: the dead letter to retry is not in the outbox document |
| MS4523 | notify | 400 | `invalid_request` | the kind must be incoming or missed | `POST /call-pushes` with another `kind` (`field` `kind`; slice G11) |
| MS4524 | notify | 400 | `invalid_request` | toPersonIds needs 1 to 49 ids of 1 to 64 characters | `POST /call-pushes` with no person, more than 49, or an empty or too long id (`field` `toPersonIds`; slice G11) |
| MS5000 | realtime | 500 | `unavailable` | Something went wrong. | an unexpected error in `realtime` |
| MS5001 | realtime | 400 | `invalid_request` | a connection id is required | the gateway connects a connection with no `connectionId` (field `connectionId`) |
| MS5002 | realtime | 400 | `invalid_request` | the kind is socket or stream | the gateway connects a connection whose `kind` is not `socket` or `stream` (field `kind`) |
| MS5003 | realtime | 400 | `invalid_request` | unknown message type | a client message whose `type` is not `auth`, `ticket`, `ping`, `typing`, `presence`, `signal`, `pairing.open`, `pairing.join`, `call.create`, `call.join`, `call.ring`, `call.ring.end` or `call.leave` (field `type`) |
| MS5004 | realtime | 400 | `invalid_request` | a conversation id is required | a `typing` message with no `conversationId` (field `conversationId`) |
| MS5005 | realtime | 400 | `invalid_request` | must name 1 to 100 members | `GET /presence` with no `memberIds` or more than 100 (field `memberIds`) |
| MS5006 | realtime | 400 | `invalid_request` | must be an RFC 3339 time | the sweep asked for presence due at a time that is not RFC 3339 (field `now`); the sweep route answers it as MS5024 |
| MS5007 | realtime | 400 | `invalid_request` | must be active, idle or dnd | a presence `state` that is not `active`, `idle` or `dnd` (field `state`) |
| MS5008 | realtime | 400 | `invalid_request` | must be an RFC 3339 time in the future | `dnd` with a `doNotDisturbUntil` that is missing or not in the future (field `doNotDisturbUntil`) |
| MS5009 | realtime | 400 | `invalid_request` | Why the envelope is refused, for example "the payload is not a JSON object of the expected shape". | retired (E12): now MS0001-MS0011; was: an event envelope refused by the shared `kit/events` (field `envelope`) |
| MS5010 | realtime | 400 | `invalid_request` | Why the type is refused, for example "the service realtime does not consume events of type member.upserted". | retired (E12): now MS0006 and MS0013; was: an event type `realtime` does not consume, or one not in the catalog, refused by the shared `kit/events` (field `type`) |
| MS5011 | realtime | 401 | `unauthenticated` | Sign in again. | the bearer token of a presence, tickets, events or sweep request is missing, forged or expired |
| MS5012 | realtime | 401 | `unauthenticated` | Sign in again. | a ticket or socket sign-in with a token that is not a valid member token (for example a service token) |
| MS5013 | realtime | 401 | `unauthenticated` | Sign in again. | a stream sign-in with a ticket that is unknown, already used or expired; the gateway closes the connection with 4401 |
| MS5014 | realtime | 401 | `unauthenticated` | Sign in again. | a `typing` or `presence` message, or a renewal, from a connection that is not signed in; the gateway closes the connection with 4401 |
| MS5015 | realtime | 401 | `unauthenticated` | Sign in again. | a message from a connection whose sign-in has ended; the gateway closes the connection with 4401 |
| MS5016 | realtime | 401 | `unauthenticated` | Sign in again. | a renewal with a token of another person or domain; the gateway closes the connection with 4401 |
| MS5017 | realtime | 403 | `forbidden` | This caller may not use this route. | the sweep caller has no `Service` role |
| MS5018 | realtime | 403 | `forbidden` | This caller may not use this route. | retired (E12): now MS0012; was: the events caller is not a service (refused by the shared `kit/events`) |
| MS5019 | realtime | 404 | `not_found` | Nothing was found. | the event's domain is not the caller's domain |
| MS5020 | realtime | 404 | `not_found` | Nothing was found. | the sweep caller is not install-wide |
| MS5021 | realtime | 404 | `not_found` | Nothing was found. | `GET /presence` for members of whom none has presence |
| MS5022 | realtime | 404 | `not_found` | Nothing was found. | `typing` in a conversation the person is not a member of |
| MS5023 | realtime | 404 | `not_found` | Nothing was found. | a connection that is not known; the gateway routes answer it as MS5014 and close the connection with 4401 |
| MS5024 | realtime | 503 | `unavailable` | Realtime is not available right now. Try again later. | realtime's storage or the connection gateway failed, or a route met an error it does not name |
| MS5025 | realtime | 400 | `invalid_request` | the socket's auth must name this device | a `signal`, `pairing.*` or `call.*` frame from a socket whose `auth` had no `deviceId` (field `deviceId`); sent as a `refused` frame |
| MS5026 | realtime | 400 | `invalid_request` | must be 1 to 64 letters, digits, - or _ | an `auth` whose `deviceId` is not 1 to 64 letters, digits, `-` or `_` (field `deviceId`); the socket gets no `auth.accepted` |
| MS5027 | realtime | 401 | `unauthenticated` | Sign in again. | an `auth` whose `deviceId` belongs to another person in `directory`; the gateway closes the connection with 4401 |
| MS5028 | realtime | 400 | `invalid_request` | must name 1 to 200 devices | a `signal` with no `toDeviceIds`, more than 200, or an id that is not 1 to 64 letters, digits, `-` or `_` (field `toDeviceIds`); sent as a `refused` frame |
| MS5029 | realtime | 400 | `invalid_request` | must be rtc, call or linking | a `signal` whose `kind` is not `rtc`, `call` or `linking` (field `kind`); sent as a `refused` frame |
| MS5030 | realtime | 400 | `invalid_request` | must be base64url text of 1 to 65536 bytes | a `signal` whose `ciphertext` is not base64url or is empty or over 64 KiB once decoded (field `ciphertext`); sent as a `refused` frame |
| MS5031 | realtime | 404 | `not_found` | Nothing was found. | a `signal` to a device that is not a device of a member of this domain (by `directory`), or, from a device not in `directory` yet, to any device but its pairing peer (field `toDeviceIds`); sent as a `refused` frame and to nobody |
| MS5032 | realtime | 400 | `invalid_request` | must be 16 to 64 letters, digits, - or _ | a `pairing.open` or `pairing.join` whose `pairingId` is not 16 to 64 letters, digits, `-` or `_` (field `pairingId`); sent as a `refused` frame |
| MS5033 | realtime | 404 | `not_found` | Nothing was found. | a `pairing.join` of a pairing that does not exist, has ended (2 minutes), belongs to another person, was opened by this device, or was joined by another device; or a `pairing.open` of an id another person or device holds (field `pairingId`); sent as a `refused` frame |
| MS5034 | realtime | 400 | `invalid_request` | must be audio or video | a `call.create` whose `media` is not `audio` or `video` (field `media`); sent as a `refused` frame |
| MS5035 | realtime | 404 | `not_found` | Nothing was found. | a `call.*` frame for a call that does not exist or has ended, or whose `callId` is not 1 to 64 letters, digits, `-` or `_` (field `callId`); sent as a `refused` frame |
| MS5036 | realtime | 404 | `not_found` | Nothing was found. | a `call.ring` or `call.ring.end` from a device that is not in the call (field `callId`); sent as a `refused` frame |
| MS5037 | realtime | 409 | `call_full` | The call already has 50 people. | a `call.join` by a new person when 50 people are in the call (field `callId`); sent as a `refused` frame |
| MS5038 | realtime | 400 | `invalid_request` | must name 1 to 49 people | a `call.ring` with no `toPersonIds`, more than 49, or a bad id (field `toPersonIds`); sent as a `refused` frame |
| MS5039 | realtime | 400 | `invalid_request` | must be base64url text of 1 to 16384 bytes | a `call.ring` whose `invite` is not base64url or is empty or over 16 KiB once decoded (field `invite`); sent as a `refused` frame |
| MS5040 | realtime | 400 | `invalid_request` | must name at most 49 people | a `call.ring.end` with more than 49 `missed` people or a bad id (field `missed`); sent as a `refused` frame |
| MS5041 | realtime | 429 | `rate_limited` | This device started 10 calls in the last minute. Wait a minute and try again. | the 11th `call.create` of one device within a minute; sent as a `refused` frame |
| MS5042 | realtime | 503 | `unavailable` | Group calls are not available right now. | `GET /calls/keys`, or a group `call.create`, when `YON_REALTIME_CALLS_KEY` (or `..._PREVIOUS`) is missing or not a P-256 PKCS #8 key; the frame gets a `refused` frame |
| MS5043 | realtime | 400 | `invalid_request` | must be 1 to 64 characters | a signalling or call frame whose `requestId` is longer than 64 characters (field `requestId`); sent as a `refused` frame |
| MS5044 | realtime | 403 | `forbidden` | This device is not linked yet. | a `call.create` or `call.join` from a device that is not in `directory` (field `deviceId`); sent as a `refused` frame |
| MS5045 | realtime | 503 | `unavailable` | Realtime is not available right now. Try again later. | a signalling or call frame met a failure of realtime's storage, the connection gateway or `directory` (or an error it does not name); sent as a `refused` frame |
| MS5046 | realtime | 401 | `unauthenticated` | Sign in again. | a re-`auth` on a socket that names another `deviceId` (or none) than its first `auth`; the gateway closes the connection with 4401 |
| MS5047 | realtime | 401 | `unauthenticated` | Sign in again. | a re-`auth` more than 5 minutes after the last owner check, when `directory` no longer lists the linked device for this person; the gateway closes the connection with 4401 |
| MS5500 | router | 502 | `function-error` | Something went wrong. | an unexpected error in the router: the Lambda endpoint did not answer, a function's answer is not a valid result, the caller left while waiting for a slot, or another error no other number names |
| MS5501 | router | 502 | `function-error` | The service failed to answer. Try again later. | Lambda answered a function call with a status other than 200 and 429, or with `X-Amz-Function-Error` |
| MS5502 | router | 429 | `function-busy` | The service is busy. Try again later. | an event push (a `POST` to a service's events route), which never waits, found every one of the function's slots in use |
| MS5503 | router | 429 | `function-busy` | The service is busy. Try again later. | Lambda answered a function call with 429 (`TooManyRequestsException`) |
| MS5504 | router | 429 | `too-many-requests` | Too many attempts. Wait and try again. | a limited route (`MESAIL_ROUTER_LIMITED_ROUTES`) was called too often from one client address; the answer has `Retry-After` |
| MS5505 | router | 404 | `unknown-service` | No service has this name. | `/api/v1/<name>/...` where `<name>` is not a known service |
| MS5506 | router | 413 | `body-too-large` | The request body is too large. | an API request body is over the router's body limit |
| MS5507 | router | 502 | `upstream-error` | The storage failed to answer. Try again later. | the static bucket (S3) answered a status other than 200, 403 and 404 |
| MS5508 | router | 404 | `not-found` | Nothing is at this path. | a static file is not in the static folder (native mode) |
| MS5509 | router | 404 | `not-found` | Nothing is at this path. | the static bucket (S3) answered 404 or 403 |
| MS5510 | router | 404 | `not-found` | Nothing is at this path. | a blocked path, or a path of a part that is off (Keycloak, blobs or dev DNS not set) |
| MS5511 | router | 400 | `bad-path` | This path is not allowed. | a static path that is not a plain file path (for example one with `..`) |
| MS5512 | router | 405 | `method-not-allowed` | Only GET and HEAD are allowed here. | a static file asked with another method |
| MS5513 | router | 405 | `method-not-allowed` | Only GET is allowed here. | the dev DNS path asked with another method |
| MS5514 | router | 405 | `method-not-allowed` | Only GET and PUT are allowed here. | a blob link used with another method |
| MS5515 | router | 502 | `upstream-error` | The service failed to answer. Try again later. | a native service server (`MESAIL_ROUTER_UPSTREAMS`) could not be reached |
| MS5516 | router | 502 | `upstream-error` | Live updates are not available now. Try again later. | a passthrough target (`MESAIL_ROUTER_PASSTHROUGH`, the realtime gateway) could not be reached |
| MS5517 | router | 502 | `upstream-error` | Sign-in is not available now. Try again later. | Keycloak could not be reached through `/idp/` |
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
| MS6024 | devicebox | 503 | `unavailable` | Too many events are waiting. Try again later. | retired (E12): now MS0017; was: `kit/outbox` refused to stage an event (`OutboxFull`) |
| MS6025 | devicebox | 404 | `not_found` | There is no such dead letter. | retired (E12): now MS0015; was: requeue names an event and subscriber that is not a dead letter |
| MS6026 | devicebox | 404 | `not_found` | There is no such outbox document. | requeue names a document that does not exist |
| MS6027 | devicebox | 404 | `not_found` | There is no such outbox collection. | requeue names a collection that is not in `outbox-collections` |
| MS6028 | devicebox | 503 | `unavailable` | The mailbox is not available. | a mailbox or job route failed in a way no other number covers |
| MS6029 | devicebox | 503 | `unavailable` | The outbox is not available. | an outbox route failed in a way no other number covers |
| MS6030 | devicebox | 503 | `unavailable` | The outbox fields cannot be read. | a document's outbox fields could not be read or written |
| MS6031 | devicebox | 503 | `unavailable` | The device list gave an answer that cannot be read. | `directory`'s owners answer is not JSON or has no `owners` |
| MS6500 | calls | 500 | `unavailable` | Something went wrong. | an unexpected error in `calls` |
| MS6501 | calls | 401 | `unauthenticated` | The join token is missing. | an offer, candidates or leave without the `X-Mesail-Join` header |
| MS6502 | calls | 401 | `unauthenticated` | The join token is not valid. | a join token that is not a signed JWT of `realtime`'s call keys, has another audience, or lacks the device or domain |
| MS6503 | calls | 401 | `unauthenticated` | The join token has expired. | an offer with a join token past its expiry |
| MS6504 | calls | 401 | `unauthenticated` | The join token is for another call. | a join token whose `callId` is not the call in the path |
| MS6505 | calls | 401 | `unauthenticated` | The join token is for another domain. | a join token whose domain is not the call's domain |
| MS6506 | calls | 409 | `call_full` | The call is full. | an offer to a call that already has 50 devices |
| MS6507 | calls | 400 | `invalid_request` | The request body is not valid JSON. | a body that is not the route's JSON model |
| MS6508 | calls | 413 | `too_large` | The request is too large. | a body over the route's size limit |
| MS6509 | calls | 400 | `invalid_request` | The offer is not valid. | an offer SDP that cannot be used (field `sdp`) |
| MS6510 | calls | 400 | `invalid_request` | A candidate is not valid. | an ICE candidate that cannot be added, or too many (field `candidates`) |
| MS6511 | calls | 404 | `not_found` | This device is not in the call. | candidates or leave from a device with no session in the call |
| MS6512 | calls | 503 | `unavailable` | The call cannot be checked now. Try again soon. | `realtime`'s call keys cannot be loaded |
