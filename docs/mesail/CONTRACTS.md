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
- **The domain is the tenant.** Every hosted mail domain is one chat space
  (DOMAIN.md). The caller's `domainId` is the `domain` claim of the Mesail
  access token (`kit/oidc` `Caller.DomainId`), which `identity` sets to the
  session's active domain (ADR 0005). No route takes the caller's domain
  from the request. Chat data never crosses a domain; email does.
- Every non-public route takes `Authorization: Bearer <access token>`, a
  Mesail access token issued by `identity` (never a Keycloak token). The
  token is checked by the service's `IdentityProvider`
  (`apps/<service>/server/controllers/TokenIdentityProvider.go`), which uses
  the shared verifier package `kit/oidc` against `identity`'s key set
  (`GET /api/v1/identity/jwks`) and the one configured issuer. Roles (empty
  structs in `apps/<service>/server/roles/`) come from the token's
  `realm_access.roles` claim (`kit/oidc` `RoleMap`), which `identity`
  fills:
  - `Member` from `mesail-member`: the person is a member of the active
    domain;
  - `DomainAdmin` from `mesail-admin`: the person administers the active
    domain, and acts only in it;
  - `Operator` from `mesail-operator`: the Keycloak realm role. It is
    install-wide; Operator routes name the target domain in the path and
    never use the caller's domain as a tenant;
  - `Service` from `mesail-service`, for calls between services (a service
    token from `identity`).
- The domain `*` (`oidc.InstallWide`) is never a hosted domain. Only an
  install-wide service token or an Operator session with no domain carries
  it, and only routes marked "install-wide" accept it.
- Errors use one model, `ApiError{code, message, field?}`. Codes are
  lowercase snake case and stable. Status codes: `400 invalid_request`,
  `401 unauthenticated`, `403 forbidden`, `404 not_found` (also for data in
  another domain or a private channel the caller is not in),
  `409 conflict` / `recipients_changed` / `duplicate` / `edit_conflict` /
  `message_deleted` / `last_admin`, `413 too_large`,
  `422 chat_not_allowed`, `429 rate_limited`, `503 unavailable`.
- **One exception, from TACHYON 26.46.02:** a request with a missing or
  forged token is answered `401` by TACHYON itself, with the plain text
  "Sign in to call this route.", before any Mesail code runs. So clients
  treat any `401` as "sign in again" (refresh once, then show the sign-in
  form), whatever its body, and never parse a `401` body.
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
never talks to Keycloak and never receives a Keycloak token. Since ADR 0005
`identity` signs people in to the one realm `mesail` and issues Mesail's
own access token, which names the active domain.

| Route | Rule | Request | Response |
| --- | --- | --- | --- |
| `POST /api/v1/identity/session` | Public | `SignInRequest{login, password, oneTimeCode?, keepSignedIn?}` | `200 Session`; `401 unauthenticated` (one message for an unknown prefix, a wrong password and a wrong code); after a correct password only: `403 not_a_member` (the address names a domain the person is not in) or `403 no_domain` (the person is in no domain and is not an Operator); `429 rate_limited` |
| `POST /api/v1/identity/session/refresh` | Public | `RefreshRequest{sessionKey}` | `200 Session` for the same active domain; `401 unauthenticated` when the Keycloak session has ended; `403 access_revoked` when the person is disabled or deleted; `403 not_a_member` when the session is alive but the active domain was removed from the person (the same `sessionKey` may still switch) |
| `POST /api/v1/identity/session/domain` | Public | `DomainSwitch{sessionKey, domainId}` | `200 Session` for `domainId` (no password); `domainId` `""` means the person's default domain (or, when that one is gone too, the first of `domainIds`); `401` and `403 access_revoked` as for refresh; `403 no_domain` when the person has no domain left and is not an Operator; `404 not_found` when the person is not a member of `domainId` |
| `DELETE /api/v1/identity/session` | Public | `RefreshRequest{sessionKey}` | `204`; ends the Keycloak session (also an offline session) |
| `DELETE /api/v1/identity/sessions` | SignedIn | - | `204`: "Sign out on all devices". Ends every Keycloak session of the caller, offline sessions included, and records `sessionsEndedAt` in `directory`, so every kept device deletes its copy at its next refresh (SPEC AC-OF-9) |
| `GET /api/v1/identity/me` | SignedIn | - | `Me{memberId, domainId, username, displayName, primaryAddress, roles[]}` from the token only (`domainId` `*` and `primaryAddress` `""` for an Operator session with no domain) |
| `GET /api/v1/identity/jwks` | Public | - | `KeySet{keys[]: PublicKey}`: the JSON Web Key Set (RFC 7517) with the current signing key and, during a rotation, the previous one. `PublicKey{kty, crv, x, y, kid, alg, use}` uses the standard JWK names (`EC`, `P-256`, `ES256`, `sig`); add them to `names.words`. No cache header: TACHYON 26.46.02 has no way to set a response header from a Go route; `kit/oidc` keeps the set for 10 s to 5 minutes. |
| `POST /api/v1/identity/service-token` | Public | `ServiceTokenRequest{clientId, clientSecret, domainId}` | `200 ServiceToken{accessToken, expiresIn}`; `401` for a wrong client or secret; `400 invalid_request` (`domainId`) when it is neither a domain name nor `*` |

`Session{me: Me, domains[]: DomainChoice{domainId, admin, default}, accessToken,
expiresIn (seconds), sessionKey, mustChangePassword, kept}`. `kept` is
true when the sign-in asked `keepSignedIn` (ADR 0006 section 2): the
`sessionKey` then seals a Keycloak offline session (scope
`offline_access`, idle 30 days, at most 90 days), and the client stores it
in its device copy instead of `@SessionState`. A refresh, a switch and
`DELETE /session` keep `kept` as it was. `domains` lists every
domain of the person, for the domain switcher, in `people.domainIds` order.
`default` is `true` on exactly one entry: the person's
`PersonRecord.defaultDomainId` (when that domain is gone, the first entry,
the one a prefix-only sign-in would open); it is `false` on every entry of
an Operator session with no domain, which has no entries. Every `Session`
answer (sign-in, refresh, switch, password change) carries it, so the
client learns a new default at its next refresh after `PUT
/directory/me/default-domain`. The client shows the default first with a
"Default" tag; the server's order stays `domainIds` order. (Decided
2026-11-11 for slice F1/L6.)

Until slice L5, `SignInRequest` also takes the old field `address` (used when
`login` is empty), and `identity` keeps the per-realm mode while its
`oidc.issuer` has `{realm}` (slice L4 builds both; L5 switches the setting).

How sign-in works (SPEC AC-ID-9):
- `login` is trimmed and lowercased. Anything outside ASCII, or a prefix
  that breaks the rules of SPEC AC-ID-11, answers the same `401` without a
  Keycloak call. The part before the last `@` is the username; the part
  after it, when present, is the wanted domain.
- `identity` calls the token endpoint of the realm `mesail` server to
  server as the confidential client `mesail-identity` (direct access
  grants, secret `YON_IDENTITY_CLIENT_SECRET`) with the username, password
  and `oneTimeCode` (Keycloak's conditional OTP).
- It reads the person with `GET /api/v1/directory/people/{personId}`
  (`personId` = the Keycloak `sub`; an install-wide service token). The
  active domain is the wanted domain, else `defaultDomainId`. When the
  wanted domain is not in `domainIds`, or the person is `disabled` or
  `deleted`, or has no domain and no Operator role, `identity` ends the
  Keycloak session it just opened and answers `403` (or `401` for
  `disabled` and `deleted`). An Operator with no domain gets the active
  domain `*`.
- **A Keycloak user without a person** (`GET /people/{personId}` answers
  `404`): `identity` ends the Keycloak session it just opened and answers
  the same `401 unauthenticated` as a wrong password, and logs one warning
  line `person_missing` with the Keycloak `sub` (never the username or
  password). This happens when a Keycloak user was made outside Mesail, or
  was deleted and made again outside Mesail: Keycloak gives the new user a
  new `sub`, and Mesail never moves an existing person's data to it
  (decided 2026-11-11). Keycloak users are made only by `directory`
  (`POST /members`); a lost Keycloak database is recovered from its backup,
  which keeps every `sub`. A later Operator tool may re-link a person to a
  new `sub` on purpose; there is none in the first release.
- `mustChangePassword` is the Keycloak user attribute
  `mesail.mustChangePassword`, which `directory` sets when it creates a
  person or resets a password; Keycloak maps it into its token for
  `mesail-identity`. `identity` clears it on `POST /identity/password`.
  The web app shows only the password change screen while it is `true`.

The Mesail access token (ADR 0005 section 3):
- JWT, ES256, `kid` = the RFC 7638 thumbprint of the current key. Keys
  from the secret store: `YON_IDENTITY_SIGNING_KEY` (a P-256 key as PKCS #8:
  base64 of the DER bytes on one line, because a TACHYON `.env` value is
  one line; a PEM text also works where the store keeps several lines) and,
  during a rotation, `YON_IDENTITY_SIGNING_PREVIOUS` (same format;
  published, never used to sign). `kit/oidc` `TokenSignerFactory` reads
  them. Not `..._KEY_PREVIOUS`: TACHYON maps `.env` names to settings by
  splitting at `_`, and `identity.signing.key` cannot be both a value and
  a folder, so `ty` refuses such a `.env`.
- Claims: `iss` (the app setting `identity.issuer`, for example
  `https://mail.example.net/api/v1/identity`), `aud` `mesail-api`, `sub`
  (person id; `service` for a service token), `azp` (`mesail-web` or
  `mesail-service`), `domain` (active domain or `*`), `realm_access.roles`
  (`mesail-member`; `mesail-admin` when the person administers the active
  domain; `mesail-operator` from Keycloak; `mesail-service` for a service
  token), `name`, `preferred_username`, `email` (`<username>@<domain>`,
  absent for `*`), `sid` (the Keycloak session id), `iat`, `nbf`, `exp`
  (`iat` + 60), `jti`.
- Every Yon app checks it with `kit/oidc` `Gate`: the signature against
  `identity`'s key set (app setting `oidc.jwks`, cached), the exact issuer
  (`oidc.issuer`), audience, `azp` (`mesail-web` or `mesail-service`),
  expiry (30 s leeway) and a non-empty `domain`. `Caller.DomainId` is the
  `domain` claim.

Sessions:
- The `sessionKey` is the Keycloak refresh token, the active domain id,
  the `kept` flag and the time of the sign-in (`signedInAt`), sealed with
  AES-GCM under `YON_IDENTITY_SESSION_KEY`. `identity` keeps no state; a
  `sessionKey` stops working when Keycloak ends the session (sign-out,
  person disabled, idle 30 minutes and at most 10 hours; for a kept
  device, idle 30 days and at most 90 days).
- **Ended elsewhere** (ADR 0006 section 2): a refresh or switch whose
  `signedInAt` is before the person's `sessionsEndedAt` answers `403
  access_revoked`, like a disabled or deleted person. `sessionsEndedAt` is
  set by `DELETE /identity/sessions`, by the person's own password change
  (`POST /identity/password`; its own fresh session starts after it), by
  the removal of one of the person's passkeys (the same way), and
  by the admin actions of SPEC AC-ID-13 (password reset, 2-step code
  removal, disable, delete). A client deletes every device copy of the
  person on `403 access_revoked`, and keeps it (locked) on `401`.
- **Refresh** refreshes the Keycloak session, reads the person again, and
  answers `403 not_a_member` when the active domain is no longer in
  `domainIds`. So removing a member cuts their access to that domain within
  60 seconds. The Keycloak session is still alive, so the client then calls
  `POST /session/domain` with the same `sessionKey` and `domainId` `""`,
  which opens the person's default domain without a password; only
  `401`, `403 access_revoked` or `403 no_domain` sends the person to the
  sign-in form. (Decided 2026-11-11 for the LU question.) These are the
  only `401` and `403` bodies a client reads: the public session routes
  are answered by `identity`, never by TACHYON's own plain-text `401`.
- **Domain switch** does the same refresh and then issues the token and a
  new `sessionKey` for the wanted domain. No password, no second Keycloak
  session.
- Tac keeps `accessToken` in memory only and sends it as
  `Authorization: Bearer <accessToken>` on every Yon call (a
  `// @RequestHeader("Authorization")` parameter). It keeps `sessionKey` in
  a `// @SessionState` field (per tab, gone when the tab closes). When a
  call answers `401`, Tac calls `POST /session/refresh` once and retries;
  when that fails, it shows the sign-in form. No timer is needed. Each tab
  has its own active domain.
- Why not cookies: TACHYON 26.46.02's Tac HTTP client always sends requests
  with `credentials: 'omit'`, so it sends no cookie and the browser ignores
  `Set-Cookie` on its responses. When TACHYON offers same-origin
  credentials, the `sessionKey` moves to an `HttpOnly` cookie (requested in
  ADR 0004).
- Realtime cannot send a header from the browser: `realtime` takes the
  access token in the first socket message (`{type:"auth", token}`), and an
  event stream takes a single-use ticket from
  `POST /api/v1/realtime/tickets` (SignedIn, 30 seconds). Never a token in a
  URL. After a domain switch the tab opens a new connection with the new
  token.
- Second factor: TOTP through Keycloak's direct-grant conditional OTP
  (`oneTimeCode`). Passkeys are a separate sign-in (below, ADR 0008).
- Brute force: Keycloak brute-force detection is on in the realm `mesail`
  (per person, so one budget for all their domains), and the gateway or
  dev router limits requests per client address (slice P1). `identity`
  keeps no counter; it answers `429` only when Keycloak reports the
  account as temporarily locked. Passwords, codes, tokens, client secrets
  and session keys never appear in logs.

Service tokens (ADR 0005 section 4):
- `POST /service-token` compares `clientId` with `mesail-service` and
  `clientSecret` with `YON_SERVICE_CLIENT_SECRET` (or
  `YON_SERVICE_CLIENT_PREVIOUS` during a rotation) in constant time
  (`kit/oidc` `ServiceClientCheck`),
  and answers a 60-second token with `azp` `mesail-service`, the role
  `mesail-service`, `sub` `service` and `domain` = the requested
  `domainId` (a domain name or `*`). It does not check that the domain is
  hosted: the receiving route does (SLICES rule 11).
- `kit/oidc` `ServiceTokens.Authorization(domainId)` calls it (app setting
  `oidc.servicetokens`, the route URL; the host is in `outgoing.hosts`) and
  caches each token per domain until 10 seconds before it expires.
  `oidc.InstallWide{}.DomainId()` (`*`) is the domain id of install-wide calls.

Passkeys (ADR 0008; Touch ID, Face ID, Windows Hello, Android). Keycloak
keeps every passkey; `identity` only relays. Built after TACHYON T13 (web)
and T14 (apps).

| Route | Rule | Request | Response |
| --- | --- | --- | --- |
| `POST /api/v1/identity/passkeys/challenge` | Public | - | `PasskeyChallenge{optionsJson, challengeKey}`: WebAuthn request options for a discoverable credential (relying party id = the install's host, user verification required, timeout 120 s) and the challenge sealed for 2 minutes |
| `POST /api/v1/identity/session/passkey` | Public | `PasskeySignIn{challengeKey, credentialJson, domainId?, keepSignedIn?}` | `200 Session` (the default domain, or `domainId` when given and the person is a member); `401 unauthenticated` (one message for every failure); `403 not_a_member`, `403 no_domain` as for a password sign-in; `429 rate_limited` |
| `GET /api/v1/identity/passkeys` | SignedIn | - | `[]Passkey{passkeyId, label, createdAt, lastUsedAt}` of the caller |
| `POST /api/v1/identity/passkeys/setup` | SignedIn | `PasskeySetupRequest{currentPassword, oneTimeCode?}` | `200 PasskeySetup{optionsJson, setupKey}` (creation options: user id = the person id, user name = the username, resident key required, user verification required, exclude the person's existing passkeys); `401` wrong password or code; `409 too_many` with 10 passkeys |
| `POST /api/v1/identity/passkeys` | SignedIn | `PasskeyConfirm{setupKey, credentialJson, label}` | `201 Passkey`; `400 setup_expired` when the `setupKey` is older than 10 minutes (start again with `setup`); `400 invalid_passkey` for every other failure |
| `PATCH /api/v1/identity/passkeys/{passkeyId}` | SignedIn | `PasskeyRename{label}` | `200 Passkey`; `400 invalid_request` (field `label`, 1 to 64 characters); `409 label_exists` when another of the person's passkeys has that label; `404 not_found` for another person's passkey. No password: a label is not a secret and changes no access |
| `DELETE /api/v1/identity/passkeys/{passkeyId}` | SignedIn | `PasskeyRemove{currentPassword, oneTimeCode?}` | `200 Session` (fresh tokens, like a password change: its own fresh session starts after `sessionsEndedAt`); ends every other session of the person (`sessionsEndedAt`); `401`; `404 not_found` for another person's passkey |

- `identity` checks the sealed challenge and its expiry, then calls
  Keycloak's token endpoint through the passkey direct-grant flow of the
  realm `mesail` (client `mesail-identity`) with the assertion, the
  expected challenge and the allowed origins (app setting
  `identity.passkeyorigins`: the web origin `https://<host>`; for the
  apps, `https://<host>` on Apple and Windows, and
  `android:apk-key-hash:<hash>` for each Android app build, printed by
  its build (TACHYON T14)). The setting is one text value with the
  origins separated by spaces (SLICES rule 17), sent to Keycloak as the
  one `passkey_origins` field. The relying party id is the host of its
  first `https://` origin; there is no second setting. The setting is
  invalid when an `https://` origin has another host, when an `http://`
  origin is not `localhost` (WebAuthn allows plain HTTP only there), or
  when no `https://` or `http://localhost` origin is listed. TACHYON
  26.46.02 has no start-up hook, so with an invalid setting every passkey
  route answers `503 unavailable` (fail closed), `GET /identity/health`
  answers `{"status":"degraded"}`, and password sign-in keeps working.
  The deploy checks `identity`'s health after each start. The relying
  party id must equal Keycloak's `MESAIL_PASSKEY_RP_ID` (PK1).
  `mesail-keycloak`'s authenticator finds the user from the user handle,
  checks signature, sign count, user verification and the origin, and
  answers tokens; a failure counts in Keycloak's brute-force detection.
- `setup` and `confirm` go through `mesail-keycloak`'s admin endpoint as
  the client `mesail-account`, only for the `sub` in the caller's token,
  like the 2-step code. The `setupKey` seals the challenge, the `sub` and
  a 10-minute expiry.
- A passkey sign-in needs no `oneTimeCode`: user verification makes it
  two factors (ADR 0008 section 1).
- `optionsJson` and `credentialJson` are strings that hold WebAuthn
  Level 3 JSON (the `toJSON()` form of `PublicKeyCredential` and of its
  options; binary values in base64url without padding), as TACHYON T13's
  `Passkeys` client takes and returns them. The user handle is the UTF-8
  bytes of the person id. Every failed passkey sign-in answers the same
  `ApiError` as a wrong password.
- Labels are 1 to 64 characters, default the device kind ("Mac",
  "iPhone"). Credential ids, public keys, challenges and assertions never
  appear in logs.
- Labels are unique per person (Keycloak refuses a duplicate, PK1). When
  adding, `identity` makes the label unique itself: if "Mac" is taken it
  stores "Mac 2", then "Mac 3", cutting the base so the result stays
  within 64 characters, and answers the stored label in `Passkey`. A
  rename to a taken label answers `409 label_exists`, because the person
  chose that name.
- `identity` to Keycloak (PK1, `deploy/keycloak/README.md` "Passkeys (ADR
  0008)"): the allowed origins go as ONE form field `passkey_origins`,
  separated by spaces, because Keycloak keeps only the first value of a
  repeated form field. Keycloak's passkey list gives `credentialId`
  (base64url) and times in milliseconds since 1970: `identity` uses the
  ids only for `excludeCredentials` in `setup` and never returns them in
  `GET /passkeys`, and answers the times as RFC 3339.

Account settings (the owner's choice: password and 2-step code are managed
in Mesail's own settings; the browser still never talks to Keycloak). They
change the person's one account, so they apply to every domain:

| Route | Rule | Request | Response |
| --- | --- | --- | --- |
| `POST /api/v1/identity/password` | SignedIn | `PasswordChange{currentPassword, newPassword, oneTimeCode?}` | `200 Session` (fresh tokens); `401` wrong current password or code; `400 password_policy` with `ApiError.message` naming the failed rule |
| `GET /api/v1/identity/totp` | SignedIn | - | `200 TotpStatus{enabled}`: whether the caller's account has an OTP credential (read through the `mesail-account` client; `Me` stays token-only) |
| `POST /api/v1/identity/totp/setup` | SignedIn | `TotpSetupRequest{currentPassword}` (no `oneTimeCode`: setup is only for an account without a 2-step code; to replace a code, `DELETE /totp` first) | `200 TotpSetup{secret, otpauthUri, qrImage, setupKey}`; `401` wrong password, or the account already has a 2-step code |
| `POST /api/v1/identity/totp/confirm` | SignedIn | `TotpConfirm{setupKey, code}` | `204`; `400 invalid_code` |
| `DELETE /api/v1/identity/totp` | SignedIn | `TotpRemove{currentPassword, oneTimeCode}` | `204`; `401` |

- **Checking the current password:** a direct grant to the realm
  `mesail` with the caller's username and current password (and code).
  Keycloak's brute-force detection counts it like a sign-in. The session
  from that grant is KEPT: it becomes the caller's fresh `Session`, for the
  caller's active domain, because Keycloak does not accept the same
  one-time code twice (agreed with slice I1).
- **Changing:** through the Keycloak admin API, signed in as the
  confidential client `mesail-account` of the realm `mesail`, whose
  service account has only the realm-management roles it needs
  (`manage-users`, `view-users`); secret `YON_IDENTITY_ACCOUNT_SECRET`.
  identity changes only
  the user whose `sub` is in the caller's token; it never takes a user id
  from the request.
- **Password:** `PUT /admin/realms/mesail/users/<sub>/reset-password`
  (`temporary: false`), then it clears the user attribute
  `mesail.mustChangePassword`, then `GET .../users/<sub>/sessions` and every
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
  in its provider (set the OTP credential of one user). Slice L2 made it
  check the realm-management role `manage-users`, so both `mesail-account`
  and `mesail-directory` may call it. That is accepted (2026-11-11): a
  client with `manage-users` can already set any credential through the
  stock admin API, so it adds no power. `identity` uses it only for the
  caller's own `sub`; `directory` never sets an OTP secret, it only removes
  one (AC-ID-13).
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
| `POST /api/v1/directory/domains` | Allow(Operator) | `CreateDomainRequest{name}` | `201 Domain` (no Keycloak change) |
| `POST /api/v1/directory/domains/{domainId}/verify` | Allow(Operator) | - | `Domain` (with `dns` states) |
| `DELETE /api/v1/directory/domains/{domainId}` | Allow(Operator) | - | `202 Domain` with `state: removing` (again `202` while it is removing); at once the domain leaves every `PersonRecord`, sign-in, refresh, switch and admin route refuse it, and mail for it is refused; the removal job then takes it off every person, releases its alias and channel-address reservations and deletes the domain (the people stay); `404 not_found` |
| `POST /api/v1/directory/domains/{domainId}/members` | Allow(Operator) | `AddMemberRequest` | as `POST /members`, for the domain in the path; used to name the first DomainAdmin (SPEC S11) |
| `POST /api/v1/directory/members` | Allow(DomainAdmin) | `AddMemberRequest{username, displayName?, firstPassword?, admin}` | `201 AddedMember{member: MemberSummary, existingPerson}`; `400 invalid_request` (`username` breaks SPEC AC-ID-11 or is a role name; `displayName` or `firstPassword` missing for a new person); `400 password_policy`; `409 duplicate` (already a member of this domain, or the prefix is an alias or channel address in any domain) |
| `PATCH /api/v1/directory/members/{memberId}` | Allow(DomainAdmin) | `MemberChange{admin}` | `200 MemberSummary`; `409 last_admin`; `404 not_found` |
| `DELETE /api/v1/directory/members/{memberId}` | Allow(DomainAdmin) | - | `204`; `409 last_admin`; `404 not_found` |
| `GET /api/v1/directory/people/{personId}` | Allow(Service), install-wide | - | `PersonRecord{personId, username, displayName, domainIds[], adminDomainIds[], defaultDomainId, state, sessionsEndedAt}`; `404 not_found` (used by `identity` on every sign-in, refresh and switch) |
| `POST /api/v1/directory/people/{personId}/sessions-ended` | Allow(Service), install-wide | - | `204`: sets `sessionsEndedAt` to now (called by `identity` for "Sign out on all devices" and the person's own password change; ADR 0006) |
| `POST /api/v1/directory/people/seed` | Allow(Service), install-wide, development only | `PersonSeed{personId, username, displayName, memberships[]: SeedMembership{domainId, admin?}}` | `200 PersonRecord`: writes the person with the given Keycloak `sub` (the fixed ids of the dev users), reserves the prefix and adds the memberships, idempotent; `404 not_found` unless the app setting `directory.developmentseed` is `true` (never in production); `409 duplicate` when the prefix belongs to another person; `400 invalid_request` (field `memberships`) for more than 2 memberships in one call, so one call stays within SLICES rule 9's sign-in-path budget (seed the person again for more domains) |
| `GET /api/v1/directory/people?query=` | Allow(Operator) | - | `PersonPage` of `PersonSummary{personId, username, displayName, domainIds[], state}` |
| `PATCH /api/v1/directory/people/{personId}` | Allow(DomainAdmin, Operator) | `PersonChange{displayName?, state?}` (`state`: `active` or `disabled`) | `200 PersonSummary`; `403 forbidden` (SPEC AC-ID-13) |
| `POST /api/v1/directory/people/{personId}/password` | Allow(DomainAdmin, Operator) | `PasswordReset{newPassword}` | `204`: sets the password, sets `mesail.mustChangePassword`, ends the person's sessions; `400 password_policy`; `403 forbidden` (AC-ID-13) |
| `DELETE /api/v1/directory/people/{personId}/totp` | Allow(DomainAdmin, Operator) | - | `204`; `403 forbidden` (AC-ID-13) |
| `DELETE /api/v1/directory/people/{personId}/passkeys` | Allow(DomainAdmin, Operator) | - | `204`: removes all the person's passkeys and sets `sessionsEndedAt`; `403 forbidden` (AC-ID-13; ADR 0008) |
| `DELETE /api/v1/directory/people/{personId}` | Allow(Operator) | - | `202`: deletes the Keycloak user and sets `state: deleted` first (so every sign-in ends; `PersonRecord.domainIds` is empty from then on), then the removal job removes every membership (`member.upserted` with `state: removed`); the prefix stays reserved for ever; calling it again answers `202`; `404 not_found` |
| `POST /api/v1/directory/jobs/removals` | Allow(Service), install-wide | - | `200 RemovalReport{writes, more}`: one step of the removal job (SLICES rule 9, sign-in path: at most 10 writes and 3 seconds per call, slice K3; `more: true` when work is left for the next call); safe to call twice at the same time. Its schedule calls it every minute (EVENTS.md section 4) |
| `PUT /api/v1/directory/me/default-domain` | SignedIn | `DefaultDomain{domainId}` | `204`; `404 not_found` when it is not one of the caller's domains |
| `POST /api/v1/directory/aliases` | Allow(DomainAdmin) | `CreateAliasRequest{address, memberId?}` | `201 Address`; `409 duplicate` when the address exists or its local part is a person's prefix (SPEC AC-ID-10) |
| `POST /api/v1/directory/channel-addresses` | Allow(DomainAdmin) | `ChannelAddressRequest{address, channelId}` | `201 Address` with `kind: channel`; the same address for the same channel again answers `201`; `409 duplicate` when a member, alias or other channel has it, or its local part is a person's prefix; `400 invalid_request` (`address`) when it is not in the caller's domain (called by `channels`) |
| `DELETE /api/v1/directory/channel-addresses/{address}` | Allow(DomainAdmin) | - | `204`; `404` unless it is a channel address in the caller's domain |
| `GET /api/v1/directory/addresses/{address}` | Allow(Service) | - | `Address{address, domainId, kind, targetId}`; `404 domain_not_hosted` when the domain is not hosted or not `active`; `404 not_found` for an unknown address in a hosted, active domain (used by the SMTP edge and `messaging`) |

`Domain{domainId, name, state, dkimSelector, dns: DnsState{mx, spf, dkim,
dmarc}}`; `state` is `pending`, `active`, `disabled` or `removing` (removing:
answered by `GET /domains` until the removal job deletes it). `domainId` is the domain name in lowercase (for example
`example.com`). `Address.kind` is `member|alias|channel`. An alias address
and its member are both in the caller's domain. `MemberSummary{memberId?,
username?, displayName, primaryAddress, admin, accountManaged}` (`username`,
`admin` and `accountManaged` only on the admin routes). `accountManaged` is
`true` when the caller may do the account actions of SPEC AC-ID-13 for this
person (reset the password, remove the 2-step code, rename, disable): the
caller is an Operator, or administers every domain of the person. The
admin app disables those actions when it is `false`; the person routes
still check it and answer `403`. (Decided 2026-11-11 for the LU
question.) Every address in a request is trimmed; the domain
part and, for hosted domains, the local part are lowercased before any
lookup (SPEC AC-ID-11).

The `dns` states come from the resolver that `directory.dnsresolver`
names. Empty (the default) means the host's own resolver. An operator may
set a DNS-over-HTTPS (JSON) URL instead; its host must then be in
`outgoing.hosts`. No public resolver is a default (ADR 0007, slice OP1).
A resolver that fails or times out (30 seconds) answers `503 unavailable`;
a missing name or record counts as `missing`.

Members and people (ADR 0005, SPEC AC-ID-10 to AC-ID-13):
- `POST /members` with a prefix that has no person: reserves the local
  part, creates the Keycloak user in the realm `mesail` (username = the
  prefix, `firstPassword` not temporary, attribute
  `mesail.mustChangePassword` `true`) through the client
  `mesail-directory` (secret `YON_DIRECTORY_ADMIN_SECRET`; Keycloak base
  URL from the app setting `directory.keycloakurl`, realm `mesail`), then
  writes `people`, `members` and `addresses` in the
  order of DOMAIN.md. With a prefix that has a person: adds the domain to
  that person; `displayName` and `firstPassword` are ignored and
  `existingPerson` is `true`. The first domain added becomes the person's
  `defaultDomainId`.
- `admin: true` adds the domain to `adminDomainIds`. `PATCH` and `DELETE`
  refuse to leave a domain without an admin (`409 last_admin`).
- The person routes check SPEC AC-ID-13 on `directory`'s data: an Operator
  always passes; a DomainAdmin passes only when every domain in the
  person's `domainIds` (at least one) is also in the caller's own
  `adminDomainIds`, and the person does not have the Keycloak role
  `mesail-operator` (so a DomainAdmin can never take over an Operator).
- When Keycloak already has a user with the new prefix but `directory` has
  no person for it, both add routes answer `409 duplicate`: `directory`
  never takes over a Keycloak user it did not create (DOMAIN "Person").
  The one exception is a user that `directory` made for an add that
  failed later (slice D2): before the Keycloak call, `directory` writes
  a new id into the prefix's `local-parts` document (`addRequest`, no
  owner yet; the same id again on a retry) and sets it on the new user
  as the attribute `mesail.addRequest`. When a later add of the same
  prefix gets "user exists" and that user's `mesail.addRequest` matches
  the document, `directory` finishes the add with that user's `sub` and
  sets the `firstPassword` of the new request. A user without the
  matching attribute still answers `409 duplicate`. While such a pending
  add exists, the prefix cannot become an alias or channel address
  (`409 duplicate`).
- Removing a domain releases its alias and channel-address reservations:
  the domain leaves each `local-parts` document with `owner: addresses`,
  and a document left without domains is deleted (also when the last
  channel address of a local part is deleted). A person's prefix is never
  released (SPEC Q5).
- `GET /members` gives `username`, `admin` and `accountManaged` when the
  caller has the role `DomainAdmin` or `Operator`, and never lists removed
  members.
- A password reset, a 2-step code removal, `state: disabled` and a
  deletion also set the person's `sessionsEndedAt`, so the person's kept
  devices delete their copies (ADR 0006).
- Every change of a person or membership publishes `member.upserted` into
  each affected domain (`state: removed` for a removed member).

Operator routes require the role `Operator` (the Keycloak realm role), from
any session of the Operator, also one with an active domain. A signed-in
request whose domain is not hosted, or is `disabled`, answers `403`, except
on Operator routes. The `Service` routes (`addresses/{address}`,
`domains/{domainId}`) accept a token for the target domain or `*`; any
other domain answers `404`. `people/{personId}`, `jobs/removals` and the outbox routes
(`POST /api/v1/directory/outbox/relay`, `GET .../outbox/dead`,
`POST .../outbox/dead/requeue`, EVENTS.md) are install-wide and accept only
`*`.

`ResolvedRecipient{address, displayName, kind:
member|channel|otherDomain|external, memberId?, channelId?, defaultDelivery:
chat|email, allowedDeliveries[]}`. The kind is decided from the caller's
domain by `kit/recipients`:

| `kind` | Meaning | `allowedDeliveries` | `defaultDelivery` |
| --- | --- | --- | --- |
| `member` | a member or alias in the caller's domain | `["chat","email","both"]` | `chat` |
| `channel` | a channel in the caller's domain | `["chat"]` | `chat` |
| `otherDomain` | hosted on this install and `active`, in another domain | `["email"]` | `email` |
| `external` | not hosted on this install | `["email"]` | `email` |

For `otherDomain` the response has no `memberId`, no `channelId` and no
display name from the other domain. **"Hosted" means `active`** (decided
2026-11-11, from L5): a domain that is `pending`, `disabled` or `removing`
is not hosted for recipients, for `mail-out`'s hand-off inside the install
or for `GET /addresses/{address}`, so mail to it goes out as `external`
mail. Members of a `pending` domain may still sign in and set it up.

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
| `GET /api/v1/messaging/conversations` | SignedIn | - | `ConversationPage` of `ConversationSummary{conversationId, kind, title, hasExternal, unreadCount, mentionCount, lastMessageAt, lastChangeId, readMessageId}` (`lastChangeId`: the newest change row of the conversation, `""` when none; `readMessageId`: the caller's read position) |
| `POST /api/v1/messaging/sync` | SignedIn | `SyncRequest{cursors[]: SyncCursor{conversationId, after}}` (1 to 50) | `200 SyncResponse{conversations[]: ConversationSync}`; see "Sync" below (ADR 0006) |
| `GET /api/v1/messaging/conversations/{conversationId}/messages` | SignedIn | `?threadRootId=` | `MessagePage` of `Message`, newest first. Without `threadRootId`: the roots of the conversation plus replies sent with `alsoToConversation`. With `threadRootId`: only the replies of that thread, never the root itself |
| `GET /api/v1/messaging/messages/{messageId}` | SignedIn (a participant) | - | `Message`; used for a thread's root and for links to one message |
| `POST /api/v1/messaging/messages` | SignedIn | `SendRequest` | `201 SendResult{message, deliveries[]}` |
| `PATCH /api/v1/messaging/messages/{messageId}` | SignedIn | `EditRequest{body, ifVersion?}` | `Message`; `409 edit_conflict` with `EditConflict{code, message, current: Message}` when `ifVersion` is not the message's `version`; `409 message_deleted` |
| `DELETE /api/v1/messaging/messages/{messageId}` | SignedIn | - | `204`, also when it is already deleted |
| `GET /api/v1/messaging/messages/{messageId}/original` | SignedIn (a participant) | - | `Download{url, expiresAt}` for the raw email as a `.eml` file; `404` for a chat message. messaging asks `GET /api/v1/mail-in/messages/{inboundId}/original` (`Allow(Service)`) for it. The client opens it as a download link and never renders it. |
| `PUT /api/v1/messaging/messages/{messageId}/reactions/{reaction}` | SignedIn | - | `204` (also when the caller already has it); `409 message_deleted` |
| `DELETE /api/v1/messaging/messages/{messageId}/reactions/{reaction}` | SignedIn | - | `204` (also when the caller does not have it); `409 message_deleted` |
| `PUT /api/v1/messaging/conversations/{conversationId}/read` | SignedIn | `ReadRequest{messageId}` | `204`; a position older than the stored one changes nothing (read positions only move forward) |
| `GET /api/v1/messaging/attachments/{attachmentId}/access` | SignedIn | - | `204` when the caller can read a message of its domain that carries the attachment, else `404` (called by `attachments` with the caller's own token). messaging answers it from one `attachment-uses` row per attachment of a message, so a message carries at most 10 attachments; more answers `400 invalid_request` with `field: attachmentIds` |

`SendRequest{clientMessageId, conversationId?, threadRootId?,
alsoToConversation, writtenAt?, body,
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
- Attachments (AC-MS-5): before it writes anything, `messaging` reads every
  `attachmentIds` entry with `GET /api/v1/attachments/{attachmentId}/meta`
  (a `Service` token of the caller's domain). An unknown id, an attachment
  of another domain or one whose `state` is not `ready` answers `400
  invalid_request` with `field: attachmentIds`; sizes that add up to more
  than 50 MiB (52,428,800 bytes) answer `413 too_large`. The message stores
  each `AttachmentRef` and answers it in `attachments[]`; `message.created`
  carries the file names in `fileNames[]`. A retry with the same
  `clientMessageId` does not ask `attachments` again.

Reactions are identified by a NAME, not an emoji character, because a Tac
path value may only use `A-Z a-z 0-9 . _ ~ -` (found by slice F2). A name
matches `[a-z0-9_-]{1,32}` and must be in the fixed list that `messaging`
accepts (in `kit/messaging`, owned by M3; first release: `thumbs_up`,
`thumbs_down`, `heart`, `laugh`, `surprised`, `sad`, `party`, `eyes`,
`check`, `pray`, `fire`, `rocket`); any other name answers `400
invalid_request` with `field: reaction`. `Message.reactions` and
`myReactions` are keyed by name; the client maps names to glyphs and
accessible labels.

`Message{messageId, conversationId, threadRootId?, alsoToConversation,
version, writtenAt?, author: Author, body, bodyFormat: markdown|text|email, blocks[]: Block,
truncated, attachments[]: AttachmentRef, email?: EmailSummary, reactions:
map[string]int, myReactions[], replyCount, lastReplyAt?, editedAt?,
deletedAt?, createdAt, deliveries[]: DeliveryStatus}`.
`Author{kind: member|otherDomain|external, memberId?, address,
displayName}`. `version` starts at 1 and goes up by one on each edit and on
the deletion. `writtenAt` is the device time a queued message was written
(ADR 0006), stored only when it is before the server's `createdAt` and at
most 30 days before it; it is shown ("Written offline at ..."), never used
for order.

**Sync** (ADR 0006 section 3, SPEC AC-OF-7):
- `messaging` appends one change row to `changes-<domainKey>-<YYYYMM>`
  for every `message.created`, `message.edited`, `message.deleted` and
  `reaction.changed` it relays, and for a thread root whose `replyCount`
  changed. Rows are kept at least 30 days.
- `ConversationSync{conversationId, state: changed|unchanged|reset|gone,
  messages[]: Message, next, more}`. For `changed`: the current full
  `Message` of every message with a change row after `after` (a message
  appears once, in its current state; deleted messages with `deletedAt`
  and no body), oldest change first, at most 200; `next` is the cursor to
  send next time; `more` is true when another call is needed. `unchanged`:
  no rows after `after`. `reset`: `after` is empty or older than the log
  keeps; the client reloads the conversation's window with `GET
  .../messages` and then uses `next` (the newest change id). `gone`: the
  caller can no longer read the conversation, or it is in another domain;
  the client deletes its copy. A conversation that does not exist answers
  `gone` too, so a batch never fails for one id.
- The server reads from 5 seconds before `after`, and the client applies
  each `Message` by `messageId` (the server's state replaces the copy), so
  a change row committed late is never missed and a repeated one changes
  nothing.
- Cursors are opaque strings. A client compares `lastChangeId` with its
  cursor only for equality.
- The realtime events `message.*` and `reaction.changed` are only a sign
  that a conversation changed: the client calls `POST /sync` for it
  rather than trusting the event (see "realtime"). `AttachmentRef{attachmentId, fileName, mediaType, size}`.
`EmailSummary{subject, from, to[], cc[], suspicious, authResults:
AuthResults{spf, dkim, dmarc, handoff}}`. `handoff` is true when another
domain of this install handed the email over (AC-DM-6); then `spf`, `dkim`
and `dmarc` are empty, and the client shows "Sent from <domain> on this
server" instead of check badges. `DeliveryStatus{recipient, channel:
chat|email, status, detail?}`.

## attachments

| Route | Rule | Request | Response |
| --- | --- | --- | --- |
| `POST /api/v1/attachments` | SignedIn | `CreateUploadRequest{fileName, mediaType, size, sha256}` | `201 Upload{attachmentId, uploadUrl, expiresAt}` |
| `POST /api/v1/attachments/{attachmentId}/complete` | SignedIn | - | `Attachment{attachmentId, fileName, mediaType, size, sha256, state}` |
| `GET /api/v1/attachments/{attachmentId}` | SignedIn | - | `Download{url, expiresAt}` (short-lived signed URL) |
| `GET /api/v1/attachments/{attachmentId}/meta` | Service | - | `Attachment{attachmentId, fileName, mediaType, size, sha256, state}` |

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
- **Meta.** For `messaging` at send time: a `Service` caller reads the
  details of an attachment of its own domain (the token's `domain`), in any
  state (`pending`, `ready` or `blocked`), to refuse a missing or not-ready
  file, to check the 50 MiB per message sum and to store `fileName`,
  `mediaType` and `size` on the message. An unknown id, or an attachment
  of another domain, answers `404 not_found`; any other caller `403
  forbidden`; no valid token `401`.
- **Blob links.** With the S3 adapter the links are AWS Signature V4
  presigned URLs (path style, `<public endpoint>/<bucket>/<domainId>/<name>`).
  The S3 adapter needs an explicit `endpoint` (`attachments.endpoint`,
  `mailin.blobs.endpoint`, the SMTP edge's
  `MESAIL_RAW_STORE_S3_ENDPOINT`); `public` falls back to it. There is no
  AWS or other cloud default (ADR 0007, slice OP1): without an endpoint
  `attachments` answers `503 unavailable`, `mail-in` refuses every blob
  call, and the SMTP edge does not start.
  With the local-folder adapter (all-in-one image, native dev) they are
  `<attachments.link.base>/<domainId>/<name>?expires&type&size&file&signature`
  (HMAC-SHA256), served by `kit/blobs` `LocalHandler` mounted by the router
  at `/blobs` with the same folder and secret as the `attachments` app
  (`PUT` answers `200`; a bad or expired signature `403`; a wrong size
  `400`).

## realtime

The browser-facing socket and event stream are served by the connection
gateway (`edge/realtime`, slice R1), the local stand-in for an AWS API
Gateway WebSocket API. The other routes are the `realtime` Yon app.

| Route | Rule | Request | Response |
| --- | --- | --- | --- |
| `GET /api/v1/realtime/socket` (WebSocket, gateway) | first message `auth` | client frames `RealtimeClientMessage` | server frames `RealtimeEvent` |
| `GET /api/v1/realtime/stream?ticket=` (event stream, gateway) | single-use ticket | - | `data:` lines of `RealtimeEvent` |
| `POST /api/v1/realtime/tickets` | SignedIn | - | `Ticket{ticket, expiresAt}` (single use, 30 seconds) |
| `PUT /api/v1/realtime/presence` | SignedIn | `PresenceRequest{state: active|idle|dnd, doNotDisturbUntil?}` | `204` |
| `GET /api/v1/realtime/presence?memberIds=` | SignedIn | - | `[]Presence{memberId, state, doNotDisturbUntil?, lastSeenAt}` |
| `POST /api/v1/realtime/events` | Service (the envelope's domain) | `events.Envelope` | `EventReceipt{eventId, status}` |
| `POST /api/v1/realtime/jobs/sweep` | Service (install-wide, domain `*`) | - | `SweepReport{closedConnections, offlineMembers, endedDoNotDisturb, deletedTickets, writes, more}` (also deletes used or expired stream tickets with the writes left of its 25) |

Socket protocol (agreed with slice F2: a Go Tac socket client gets no open
or close callback and has no timer):
- On every new connection, reconnects included, the server first sends
  `RealtimeEvent{type: "hello"}`. The client answers with `auth`. The
  server sends no other event before an accepted `auth`; without one within
  10 seconds it sends `auth.expired` and closes with 4401.
- Client frames: `RealtimeClientMessage{type, token?, conversationId?,
  state?, doNotDisturbUntil?}` with `type` one of `auth` (`token`: the
  access token; the first message, within 10 seconds), `typing`
  (`conversationId`), `presence` (`state`, `doNotDisturbUntil?`) and
  `ping` (keeps an idle socket open; send one every 5 minutes).
- After each accepted `auth` the server sends
  `RealtimeEvent{type: "auth.accepted", state: <RFC 3339 time>}`. The
  socket closes with code 4401 at that time unless the client sent `auth`
  again with a newer token. The time is the token's expiry, at most 60
  seconds ahead, so the token is re-checked at least every 60 seconds
  (SPEC AC-ID-4). 20 seconds before that time the server sends
  `RealtimeEvent{type: "reauth"}`; the client answers with `auth` and a
  fresh token. A refused `auth`, or `typing`/`presence` before an accepted
  `auth`, also ends the socket.
- Before such an end the server sends
  `RealtimeEvent{type: "auth.expired", state: "4401"}`, then closes with
  code 4401. The client treats `auth.expired` like 4401 (get a fresh token,
  reconnect), because an API Gateway WebSocket API (the later serverless
  form) can only close with 1000 or 1001.
- The event stream sends the same events. It ends with `event: close` and
  `data: {"code":4401}` when the token behind the ticket ends; the client
  then gets a new ticket and reconnects.

Presence is per domain: a caller sees presence only of members in its own
domain, and member ids from another domain are left out of the answer. When
none of the asked ids is found the answer is `404`, so a query for a member
of another domain gets `404` (SPEC AC-PN-1). The socket sends only events
of the caller's domain. Typing goes only to the conversation's members.

`RealtimeEvent{type, conversationId, messageId, memberId?, state}` carries
ids only, never message bodies; the client fetches what it needs. A client
with a device copy (ADR 0006) treats `message.*` and `reaction.changed`
as a sign to call `POST /api/v1/messaging/sync` for that conversation (at
most once per second per conversation), and starts a full catch-up after
every reconnect, because events sent while it was away are not replayed.
`RealtimeEvent.type` is one of `message.created`, `message.edited`,
`message.deleted`, `reaction.changed` (`state` = the emoji),
`read.moved` (only to the reader's own connections), `presence.changed`,
`typing`, `notification`, `hello`, `reauth`, `auth.accepted` and
`auth.expired`.

Internal routes between the gateway and the Yon app (never routed
publicly; shared secret in the header `X-Realtime-Gateway`):
`POST /api/v1/realtime/gateway/{connect,message,disconnect}` (role
`Gateway`, the `$connect`, `$default` and `$disconnect` of API Gateway) and
the gateway's management API `POST` / `DELETE /@connections/{connectionId}`
(API Gateway's `@connections`). Details: `edge/realtime/README.md`.

## notify

| Route | Rule | Request | Response |
| --- | --- | --- | --- |
| `GET /api/v1/notify/notifications` | SignedIn | - | `NotificationPage` of `Notification{notificationId, kind, conversationId, messageId, createdAt, readAt?}` |
| `PUT /api/v1/notify/notifications/{notificationId}/read` | SignedIn | - | `204` |
| `POST /api/v1/notify/push-subscriptions` | SignedIn | `PushSubscriptionRequest{endpoint, p256dh, auth}` | `201` |
| `GET /api/v1/notify/settings` / `PUT` | SignedIn | `NotifySettings{emailSummaryAfterMinutes (0=off, 15..1440), mutedConversationIds[]}` | `NotifySettings` |
| `DELETE /api/v1/notify/push-subscriptions` | SignedIn | `PushSubscriptionRequest` (only `endpoint` is used) | `204`; only the caller's own subscription |
| `GET /api/v1/notify/push-key` | SignedIn | - | `PushKey{publicKey}` (the VAPID public key for `pushManager.subscribe`); `503 unavailable` when not configured or when `notify.push.enabled` is `false` (the web app then hides push; ADR 0007, SPEC Q19) |
| `POST /api/v1/notify/events` | Allow(Service) | `Envelope` | `EventReceipt` (EVENTS.md) |
| `POST /api/v1/notify/outbox/relay` | Allow(Service) | - | `OutboxReport` |
| `POST /api/v1/notify/jobs/run` | Allow(Service), install-wide (domain `*`) | - | `JobReport{domains, fannedOut, pushed, skipped, summarized, more}` |
| `GET /api/v1/notify/messages/{messageId}/source` | Allow(Service) | - | `MessageSource{messageId, domainId, body, attachmentIds[]}` of an email summary (`sum_` id) |

How it works:
- Notifications (AC-PN-2) come from `message.created`: a mention, a direct
  message (`kind` `email` when the direct message is an email), or a reply
  in a thread the member wrote in. Only members who can read the message
  get one, never the author. A muted conversation keeps only mentions.
- Web Push (AC-PN-3) uses VAPID (RFC 8292) and `aes128gcm` encryption (RFC
  8291). The encrypted payload is the JSON that TACHYON's push worker shows
  (TACHYON ADR 0118): `{"title", "body", "path", "tag"}`. `title` is the
  sender's display name; `body` says what happened and where, with no
  message text ("mentioned you in #design (example.com)", "sent you a
  message (example.com)", "replied in a thread (example.com)"); `path` is
  the web app's path that opens the conversation in its domain, built only
  from the domain id and the conversation id (the web slice names the
  route); `tag` is `<domainId>/<conversationId>`, so a newer push for the
  same conversation replaces the older one. A subscription is stored per
  (domain, member); the web app registers the same browser subscription
  once in each domain it opens while push is on. Push is off unless
  `notify.push.enabled` is `true` (default `false`: on-premise installs
  reach no push service unless an Operator turns it on; ADR 0007, SPEC
  Q19). While it is off, `GET /push-key` answers `503 unavailable` and no
  Web Push is sent: a waiting push is marked `skipped`. The VAPID keys come from the secret store (`.env`:
  `YON_NOTIFY_VAPID_PUBLIC_KEY`, `YON_NOTIFY_VAPID_PRIVATE_KEY`,
  `YON_NOTIFY_VAPID_SUBJECT`). A push endpoint must be `https` on a known
  push service host (`notify.push.hosts`), else `400 invalid_request`.
  Nothing is pushed while the member's presence is `dnd` or `active`; the
  service worker still decides whether a tab is focused. The push text never
  holds message text. A subscription the push service reports gone (404,
  410) is deleted. Native push targets (APNs, FCM) plug in behind the same
  `webpush.Target` seam later.
- The email summary (AC-PN-4) is sent once per batch of unread
  notifications after the member's delay, as `email.send.requested` from
  `notifications@<domain>` to the member's primary address (producer
  `notify`). mail-out reads its body from
  `GET /api/v1/<envelope producer>/messages/{messageId}/source`.
- Everything is per domain: the caller's domain for routes, the envelope's
  `domainId` for events. A notification of another member or domain
  answers `404`.

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
  `mail-in` builds and keeps it when it accepts the message, and sends the
  same content in `email.received` (`content`), so `messaging` stores the
  blocks on the Message without calling `mail-in` back (EVENTS.md section
  4, item 4). `messaging` reads this route only for an older event that has
  no `content`. An accept whose raw message cannot be read answers
  `503 unavailable` and stages no event. For an inline image of an email, `ImageRef.attachmentId`
  is the `kit/blobs` key of the stored part (`<domainId>/attachment-<sha256 hex>`),
  the same value as that part's `InboundAttachment.blobKey`. A message that
  `kit/mime` cannot parse (malformed, or over its 25 MiB limit) answers empty
  `blocks` and `attachments` with `truncated: true`, so the client offers the
  original. Size budget (`kit/mailin` ContentBudget), the same for this route
  and for the event: at most 256 KiB of JSON, so every FYLO document that
  holds it fits FYLO's 1 MiB request frame. `text` is cut at 200,000
  characters; content still over 256 KiB keeps the first 64 KiB of text
  with blocks made from it (`bodyFormat: text`); content still over that
  becomes the unreadable form above with the first 16 KiB of text. Each cut
  sets `truncated: true`. `messaging` makes the blocks for chat messages
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
    The check uses an install-wide service token (domain `*`), because the sender domain may not be hosted; an
    unknown or inactive sender domain answers `400 invalid_request`
    (field `envelopeFrom`), a directory failure `503`. The copy's key is
    `<recipientDomain>/inbound-<sha256 hex of the sender key>`.
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
  headers: EmailHeaders, authResults: {spf, dkim, dmarc, handoff},
  content: EmailContent}`
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
slice M3; for an event whose producer is `notify`, an email summary, from
`GET /api/v1/notify/messages/{messageId}/source` instead), reads `dkimSelector` and the hosted state from `directory`
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
