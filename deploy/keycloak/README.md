# Keycloak for Mesail

Keycloak (OpenID Connect) replaces SESAME as Mesail's identity provider
([ADR 0004](../../docs/adr/0004-mesail-go-serverless-keycloak.md), section 6).
This folder builds Keycloak **26.8.2** into the `mesail-keycloak` stage of the
repository `Dockerfile`, with **SQLite** as its database.

Keycloak does not support SQLite. It runs on SQLite here because of a small
provider jar in `providers/sqlite`. Read [Limits](#limits) before you use it
in production.

## Layout

| Path | What it is |
| --- | --- |
| `conf/keycloak-sqlite.conf` | Build and runtime options for the SQLite image (the default). |
| `conf/keycloak-postgres.conf` | Options for the PostgreSQL image (`--build-arg KEYCLOAK_DB=postgres`). |
| `conf/quarkus.properties` | Turns off Hibernate's database version check (it expects PostgreSQL 14+), and records each request's start time so the access log can show its duration. |
| `providers/sqlite/` | Java source of `mesail-keycloak-sqlite.jar` (SQLite support), compiled in the `keycloak-providers` stage. Left out of the PostgreSQL image. |
| `providers/mesail/` | Java source of `mesail-keycloak-mesail.jar`: the admin API endpoints that store a TOTP and manage passkeys, and the passkey sign-in step of the direct grant (see [Passkeys](#passkeys-adr-0008)). In both images. |
| `realm/mesail-realm.json` | The one realm `mesail`: every person and every Operator (ADR 0005). |
| `realm/dev/mesail-users-0.json` | Dev-only people `ana`, `bo`, `eve`, `carol` and `operator`, with fixed Keycloak ids `3e5a11d0-0000-4000-8000-00000000000N` (N = 1 to 5 in that order). Imported only with `MESAIL_KEYCLOAK_DEV_USERS=true`. |
| `bin/mesail-supervisor` | The image entrypoint. Starts Keycloak, waits until it is ready, then starts `MESAIL_APP_COMMAND`. |
| `bin/mesail-keycloak-start` | Checks the environment, prepares the realm `mesail` (and the dev users when asked), chooses the database, runs `kc.sh start --optimized --import-realm`. |
| `dev/build-native-keycloak.sh` | Builds the same Keycloak natively on macOS or Linux (no Docker), for development. |
| `bin/mesail-healthcheck` | Docker `HEALTHCHECK`: Keycloak readiness, plus `MESAIL_APP_HEALTH_URL` when set. |
| `test/run-image-test.sh` | Builds the image and proves it works (see [Testing](#testing)). |
| `test/oidc_check.py` | The checks of the realm `mesail` (Python standard library and `openssl`). |
| `test/keycloak_client.py` | HTTP, token, TOTP and admin API helpers of the check. |
| `test/passkey_check.py` | The passkey checks, with `test/software_authenticator.py` (an ES256 key made with `openssl`, a made-up credential id, user verification set) and `test/cbor_writer.py`. |

## The realm `mesail`: one login per person

Since [ADR 0005](../../docs/adr/0005-one-login-per-person.md) Keycloak has one
realm, `mesail`. It holds every person: members of any mail domain and the
Operators. Keycloak owns only credentials and sessions: the password, the
2-step code (TOTP), brute-force detection, the session lifetime and the realm
role `mesail-operator`. Which domains a person belongs to, and whether they
administer one, is `directory` data, not Keycloak data. Adding a domain does
not touch Keycloak.

**Keycloak is reached only from the backend.** No browser talks to it, and
`/idp` is not routed from the public address. The identity service signs
people in server to server and gives the browser Mesail's own token;
Keycloak's tokens never leave the identity service.

**Usernames.** The username is the person's email prefix, stored in lowercase
(`ana` for `ana@example.com` and `ana@example.org`). The realm's user profile
allows 1 to 64 characters of `a-z`, `0-9`, `.`, `_` and `-`, starting and
ending with a letter or digit, with no two dots in a row (SPEC AC-ID-11); the
admin API answers 400 for anything else and 409 for a prefix that exists.
`editUsernameAllowed` is false. Keycloak lowercases and trims the username at
sign-in, so `ANA` and ` ana ` sign in as `ana`. Sign-in with an email address
is off in Keycloak (`loginWithEmailAllowed` false): the identity service turns
`ana@example.com` into `ana` itself. Emails are optional and may repeat
(`duplicateEmailsAllowed` true); the dev users have their default address.

| Client | Type | Use |
| --- | --- | --- |
| `mesail-identity` | confidential, password grant only | The identity service signs a person in, with a password or with a passkey. The realm's direct grant flow `mesail direct grant` asks for a TOTP code (`totp` form field) when the person has one set up and signs in with a password. Its access token has `aud` `mesail-identity` only (never `mesail-api`, so no service accepts it), the realm role `mesail-operator` for Operators, and the claim `mesail_must_change_password` (boolean, from the user attribute `mesail.mustChangePassword`; absent when the attribute is not set). |
| `mesail-account` | confidential, client credentials; service account with `realm-management` `view-users` and `manage-users` only | The identity service changes a person's own settings: password, TOTP, sessions, the `mesail.mustChangePassword` attribute, lock status and unlock. 403 on anything else, and on other realms. |
| `mesail-directory` | confidential, client credentials; service account with `realm-management` `view-users`, `query-users` and `manage-users` only | The directory service creates, finds, disables and deletes people, sets `mesail.mustChangePassword`, resets passwords, removes a TOTP and logs a person out. It cannot create realms or read clients. |

There is no `mesail-service` client any more: service tokens come from the
identity service (ADR 0005 section 4). There is no browser client. Keycloak's
own `account-console` and `security-admin-console` clients still exist
(Keycloak always creates them), but nothing routes browsers to them.

**Roles.** The only Mesail realm role is `mesail-operator`, given to each
Operator explicitly. The default role `default-roles-mesail` is empty, so a
person's token and the two service accounts' tokens carry no other realm
role. `mesail-member`, `mesail-admin` and `mesail-service` are roles of
Mesail's own token, set by the identity service.

**Fixed dev ids.** A person's id is their Keycloak `sub`, and it never
changes: `directory` never links a person to a new `sub` (ADR 0005, DOMAIN
"Person"). Every dev user file therefore sets each user's `id`, so a fresh
Keycloak database gives the same subjects as the last one and matches the
directory's dev seed: `ana` `...0001`, `bo` `...0002`, `eve` `...0003`,
`carol` `...0004`, `operator` `...0005` (all `3e5a11d0-0000-4000-8000-00000000000N`).
Keycloak user ids are unique, so a new dev user needs an id no other dev
user has. Real people
get Keycloak's random ids from `directory`.

**User attributes.** The realm's user profile declares two attributes;
only admins see or change them, and Keycloak drops any attribute it does not
declare:
- `mesail.mustChangePassword` (`true` or `false`), mapped into the
  `mesail-identity` token as `mesail_must_change_password`;
- `mesail.addRequest` (at most 64 characters; the directory writes 32
  lowercase hex characters), the id of the directory's add request. It has no
  protocol mapper, so it never reaches a token;
- `mesail.passkeyLastUsed` (many values, each `<passkey id> <milliseconds>`),
  when each passkey last signed the person in. The passkey sign-in step writes
  it; the passkey endpoints remove a value with its passkey. It has no
  protocol mapper. A client that writes the person back with `PUT
  .../users/<id>` keeps the values it read.

Profile verification (`VERIFY_PROFILE`) is off and no field but the username
is required, so a person made through the admin API without an email or a
name can sign in with the password grant.

**Admin API calls** (all tested, in the realm `mesail`, with a
client-credentials token of `mesail-account` or `mesail-directory`):

| Action | Call | Client |
| --- | --- | --- |
| Find a person | `GET /admin/realms/mesail/users?username=<prefix>&exact=true` | both |
| Create a person | `POST /admin/realms/mesail/users` with `username`, `firstName`, `lastName`, `enabled`, `attributes.mesail.mustChangePassword` `["true"]` and a password credential (`temporary: false`) | directory |
| Mark a new person with the directory's add request, and read it back after a 409 on create (SLICES D2: finish a half-finished add only when it matches the prefix's reservation) | `attributes.mesail.addRequest` `["<32 hex>"]` in the create body; `GET /admin/realms/mesail/users?username=<prefix>&exact=true` then `GET .../users/<id>` | directory |
| Set or clear must-change | `PUT /admin/realms/mesail/users/<id>` with the user from `GET` and the changed `attributes` | both |
| Disable or enable | `PUT .../users/<id>` with `enabled` | directory |
| Delete | `DELETE .../users/<id>` | directory |
| Set up TOTP with a secret the identity service made, after the person typed a code | `PUT /admin/realms/mesail/mesail/users/<id>/otp` (see below) | account |
| List or remove TOTP | `GET .../users/<id>/credentials`, `DELETE .../users/<id>/credentials/<credential id>` | both |
| Change the password | `PUT .../users/<id>/reset-password` with `{"type":"password","value":"...","temporary":false}` | both |
| List or end sessions | `GET .../users/<id>/sessions`, `DELETE /admin/realms/mesail/sessions/<session id>` | account |
| Log out everywhere | `POST .../users/<id>/logout` | both |
| Lock status, unlock | `GET` / `DELETE /admin/realms/mesail/attack-detection/brute-force/users/<id>` | account |
| Add, list, rename or remove one passkey | `/admin/realms/mesail/mesail/users/<id>/passkeys[/<passkey id>]` (see [Passkeys](#passkeys-adr-0008)) | account |
| Remove all of a person's passkeys | `DELETE /admin/realms/mesail/mesail/users/<id>/passkeys` | directory |

**The TOTP endpoint** (`providers/mesail`, Keycloak's admin REST extension
SPI, so Keycloak's own admin authentication runs first). It:

- requires `manage-users` for that user (Keycloak's permission check), so
  `mesail-account` and `mesail-directory` may call it; a person's own token
  gets 401 or 403;
- requires digits, period and algorithm equal to the realm's OTP policy
  (6, 30, `HmacSHA1`), and a base32 secret of 160 to 640 bits;
- checks the code against the secret with the realm's look-around (one 30 s
  step either side);
- accepts a code only once for this user, and marks it used for sign-in too, so
  the confirmation code cannot sign in afterwards;
- refuses a second TOTP (remove the old one first);
- stores the credential and writes an admin event (`CREATE` on
  `users/<id>/credentials/<credential id>`, without the secret; admin events
  are on in the realm) and a log line.

Answers: 204 stored; 400 `{"error":"invalid_request"}`, `invalid_code` or
`code_used`; 401 or 403 without the right; 404 `user_not_found`; 409
`otp_exists`. The stock `PUT /users/<id>` with an `otp` credential also stores
a TOTP, but checks no code; Mesail uses the endpoint.

**Sign-in answers.** A wrong password, a missing or wrong TOTP code, a
disabled person and a locked account all get the same answer: HTTP 400
`invalid_grant`. Keycloak does not say which (so nobody can probe accounts).
The identity service can ask for the lock status with `mesail-account`.
Lockout counts per person (one budget for all their domains): 10 failures
lock the account for up to 15 minutes, and two failures within 1 second lock
it for 60 seconds (Keycloak's quick login check).

**TOTP:** SHA-1, 6 digits, 30 seconds; a code one step before or after the
current one is accepted, and each code signs in only once.

Tokens: RS256, access token 60 seconds, refresh token rotation on (a refresh
token works once), SSO idle 30 minutes, at most 10 hours. Passwords need 12
characters and may not equal the username or the email. Self-registration and
password reset by email are off.

**Issuer:** `${MESAIL_KEYCLOAK_URL}/realms/mesail`. `MESAIL_KEYCLOAK_URL` is
the backend address of Keycloak, default `http://127.0.0.1:8180/idp` (the
all-in-one image). If Keycloak runs as its own host, use that host, for
example `http://keycloak:8180/idp`. Only the identity service reads this
issuer.

The realm file uses the placeholders `${MESAIL_IDENTITY_CLIENT_SECRET}`,
`${MESAIL_ACCOUNT_CLIENT_SECRET}`, `${MESAIL_KEYCLOAK_DIRECTORY_SECRET}` and
`${MESAIL_PASSKEY_RP_ID}`,
filled by Keycloak from the environment. The import runs only when the realm
does not exist yet, so changing a secret later changes nothing: change it
through the admin API.

## Passkeys (ADR 0008)

A passkey (Touch ID, Face ID, Windows Hello, an Android biometric) is a
WebAuthn credential of type `webauthn-passwordless` of the person in Keycloak.
Keycloak keeps its public key and sign count; Mesail stores no credential.
Passkeys are an extra way to sign in: every person keeps the password.
The browser never talks to Keycloak: the identity service relays every
challenge, creation and assertion.

**Policy** (the realm's WebAuthn passwordless policy): relying party id
`MESAIL_PASSKEY_RP_ID` (the install's public host name, for example
`mail.example.com`); user verification required; resident key required;
attestation `none` (any attestation format is accepted and none is trusted);
ES256 and RS256; at most 10 passkeys per person (checked by the provider).
The relying party id is read when the realm is imported, on the first start.
To change it later, change `webAuthnPolicyPasswordlessRpId` of the realm
through the admin API; passkeys made for the old host stop working.

**Sign-in** (direct grant, client `mesail-identity`). The realm's direct grant
flow is `mesail direct grant`: first the step `mesail-passkey-grant`, then the
password steps (username, password, and the TOTP code when the person has one;
the same steps as Keycloak's own direct grant flow). When the request has a
`passkey_assertion` field, only the passkey step runs; without it, only the
password steps run.

```
POST /idp/realms/mesail/protocol/openid-connect/token
grant_type=password  client_id=mesail-identity  client_secret=...  scope=openid
passkey_assertion=<credentialJson: the PublicKeyCredential toJSON() of navigator.credentials.get>
passkey_challenge=<the challenge the identity service made, base64url, 16 to 64 bytes>
passkey_origins=<the allowed origins, separated by spaces>
```

`passkey_origins` lists `https://<host>` (web, Apple and Windows apps) and
`android:apk-key-hash:<hash>` for each Android app build (the identity
setting `identity.passkeyorigins`); `http://localhost` and `http://127.0.0.1`
(with any port) are also accepted for development. At most 16. It is one
field because Keycloak keeps only the first value of a repeated form field.

The step finds the person from the assertion's user handle (the UTF-8 bytes
of the person's Keycloak id), refuses a disabled or locked person, accepts
each challenge once (for 5 minutes), and checks the assertion with
Keycloak's own WebAuthn code: the signature with the stored public key, the
challenge, the origin, the relying party id hash, user presence, user
verification (always required) and the sign count. On success Keycloak
answers tokens like a password sign-in, with no TOTP code (user verification
counts as the second factor), and `mesail.passkeyLastUsed` is updated.

Every failure (a malformed request, an unknown user handle, an unknown or
removed passkey, a wrong signature, origin or challenge, no user
verification, a replayed assertion, a disabled or locked person) gets the same
answer as a wrong password: HTTP 400
`{"error":"invalid_grant","error_description":"Invalid user credentials"}`.
A failure of a known person counts in brute-force detection like a wrong
password (Keycloak itself does not count WebAuthn failures), so 10 failed
passwords and passkeys together lock the person; a passkey sign-in resets the
count like a password sign-in.

**Admin endpoints** (`providers/mesail`, under
`/admin/realms/mesail/mesail/users/<person id>/passkeys`; Keycloak's admin
authentication runs first). Listing needs `view-users`; every change needs
`manage-users`. A person's own token gets 401 or 403. Every change writes an
admin event (`CREATE`, `UPDATE` or `DELETE` on
`users/<id>/credentials/<passkey id>`) with the type, the passkey id and the
label, never the key or the credential id. A passkey is answered as
`{"passkeyId", "credentialId", "label", "createdAt", "lastUsedAt"}`:
`passkeyId` is Keycloak's credential id, `credentialId` the WebAuthn
credential id (base64url, for `excludeCredentials`), the times are
milliseconds since 1970 and `lastUsedAt` is `null` before the first sign-in.

| Call | Body | Answers |
| --- | --- | --- |
| `POST .../passkeys` (add) | `{"credentialJson": "<toJSON() of navigator.credentials.create>", "challenge": "<base64url>", "origins": ["https://<host>", ...], "label": "Mac"}` | `201` the passkey; `400 invalid_request` (body, label, origins or challenge), `400 invalid_passkey` (the attestation does not check: origin, challenge, relying party, user verification, algorithm), `400 challenge_used`; `404 user_not_found`; `409 too_many` (10 already), `409 passkey_exists` (the person has this credential), `409 label_exists` (the person has a passkey with this label) |
| `GET .../passkeys` (list) | - | `200` the person's passkeys; `404 user_not_found` |
| `PUT .../passkeys/<passkey id>` (rename) | `{"label": "MacBook"}` | `200` the passkey; `400 invalid_request`; `404 not_found` (not a passkey of this person); `409 label_exists` |
| `DELETE .../passkeys/<passkey id>` (remove) | - | `204`; `404 not_found` |
| `DELETE .../passkeys` (remove all; the directory, AC-ID-13) | - | `204` |

Labels are 1 to 64 characters with no control characters, trimmed; they are
unique per person (Keycloak's rule for credential labels). Removing passkeys
does not end Keycloak sessions: the identity and directory services end the
person's other sessions themselves (`sessionsEndedAt`).

## Running the image

```bash
docker build --target mesail-keycloak -t mesail-keycloak .
```

```bash
docker run -d --name mesail-keycloak -v mesail-keycloak:/data/keycloak \
  -e MESAIL_IDENTITY_CLIENT_SECRET=... -e MESAIL_ACCOUNT_CLIENT_SECRET=... \
  -e MESAIL_KEYCLOAK_DIRECTORY_SECRET=... \
  -e MESAIL_APP_COMMAND='/path/to/mesail-front' mesail-keycloak
```

| Variable | Required | Meaning |
| --- | --- | --- |
| `MESAIL_IDENTITY_CLIENT_SECRET` | yes | Secret of `mesail-identity` (the identity service's `YON_IDENTITY_CLIENT_SECRET`). Used when the realm is imported. |
| `MESAIL_ACCOUNT_CLIENT_SECRET` | yes | Secret of `mesail-account` (the identity service's `YON_IDENTITY_ACCOUNT_SECRET`). |
| `MESAIL_KEYCLOAK_DIRECTORY_SECRET` | yes | Secret of `mesail-directory` (the directory service's `YON_DIRECTORY_ADMIN_SECRET`). |
| `MESAIL_KEYCLOAK_URL` | no | Keycloak's backend address and the issuer base. Default `http://127.0.0.1:8180/idp`. |
| `MESAIL_PASSKEY_RP_ID` | no, but set it in production | The passkey relying party id: the install's public host name, the same host the identity service names in its WebAuthn options. Read when the realm is imported. Default `localhost`, with a warning. |
| `KC_BOOTSTRAP_ADMIN_CLIENT_ID`, `KC_BOOTSTRAP_ADMIN_CLIENT_SECRET` | no | A temporary admin client of the `master` realm, for admin work by hand and for the admin event check in `test/oidc_check.py`. Remove it from the environment after use. |
| `KC_BOOTSTRAP_ADMIN_USERNAME`, `KC_BOOTSTRAP_ADMIN_PASSWORD` | no | A temporary admin user of the `master` realm, for the admin console (reach it through a tunnel, never the public address). |
| `MESAIL_APP_COMMAND` | no | The app the supervisor starts after Keycloak is ready. Without it the container runs Keycloak only. |
| `MESAIL_APP_HEALTH_URL` | no | `http://127.0.0.1:<port>/<path>` of the app's health route, added to the image health check. |
| `MESAIL_KEYCLOAK_DATA` | no | Folder of the SQLite file. Default `/data/keycloak` (a volume). |
| `MESAIL_KEYCLOAK_IMPORT_DIR` | no | Another folder of realm files laid out like `realm/`. |
| `MESAIL_KEYCLOAK_DEV_USERS` | no | `true` imports the dev people of the realm `mesail`: `ana`, `bo`, `eve`, `carol` (password `dev-<name>-password`) and `operator` (`dev-operator-password`, role `mesail-operator`). Their domains are `directory` data. Dev and tests only. |
| `MESAIL_KEYCLOAK_START_TIMEOUT` | no | Seconds the supervisor waits for Keycloak (default 300). |

Inside the container Keycloak listens on `127.0.0.1:8180` under `/idp`, and
its health endpoints on `127.0.0.1:9180/idp/health/*`. Only processes in the
same container reach it. The front must not route `/idp`. When Keycloak runs
as its own host for other services, set `KC_HTTP_HOST=0.0.0.0` and keep port
8180 on a private network.

**Supervisor.** `mesail-supervisor` is the entrypoint (process 1). It starts
Keycloak, waits up to `MESAIL_KEYCLOAK_START_TIMEOUT` for readiness, then
starts the app. When either process ends, it stops the other and the container
exits with that status, so the platform restarts the whole unit. On `SIGTERM`
it stops the app first, then Keycloak (10 seconds each); give the container a
stop timeout of at least 25 seconds.

## How Keycloak runs on SQLite

Keycloak 26.8.2 accepts only its own list of databases, and SQLite cannot run
most of Keycloak's schema migrations. Each piece below fixes one failure that
was seen when running it:

1. **Driver and dialect.** `sqlite-jdbc` 3.54.1.0 (checksum-pinned) is added
   to `/opt/keycloak/providers`. The build uses `db=postgres` only as the vendor
   slot, with `db-driver=org.sqlite.JDBC` and the hidden option
   `db-dialect=org.hibernate.community.dialect.SQLiteDialect` (Keycloak already
   ships Hibernate's community dialects).
2. **Migration lock.** Keycloak's lock runs `SELECT ... FOR UPDATE`, which SQLite
   rejects. The `sqlite` lock provider (`spi-dblock--provider=sqlite`) locks
   inside the JVM instead. That is correct only because one Keycloak process
   owns the file.
3. **Liquibase database type.** Keycloak gives Liquibase the PostgreSQL
   database type of the vendor slot and removes Liquibase's SQLite type at build
   time. The `sqlite` Liquibase connection provider
   (`spi-connections-liquibase--provider=sqlite`) gives it `SQLiteDatabase`.
4. **Schema changes SQLite cannot do with `ALTER TABLE`:**
   - add primary key, add unique constraint: a unique index with the
     constraint's name (same uniqueness rule);
   - add foreign key: nothing, except `ON DELETE CASCADE`, which becomes a
     trigger on the parent table; drop foreign key drops that trigger;
   - modify data type, drop default value: nothing (SQLite stores any type);
   - drop column, add or drop NOT NULL, drop primary key: the table is rebuilt
     from SQLite's own schema (`pragma_table_info`, `sqlite_master`), keeping
     its rows, indexes and triggers;
   - add column: SQLite's own `ALTER TABLE ... ADD COLUMN`.
   Liquibase's own SQLite rebuild is not used: it fails on Keycloak's schema
   (a null primary-key flag, and a schema snapshot query over SQLite's limit of
   500 `UNION` terms).
5. **Schema snapshot.** Liquibase's "column exists" checks ask the driver for
   the columns of all tables at once, which hits the same 500-term limit. The
   provider answers that one call table by table.
6. **Locking.** The database URL sets `journal_mode=WAL`,
   `transaction_mode=IMMEDIATE`, `busy_timeout=15000` and
   `synchronous=NORMAL`. Every transaction takes SQLite's write lock when it
   begins, so transactions never fail half-way with `SQLITE_BUSY_SNAPSHOT`;
   they wait for each other instead.
7. **No clustering.** `cache=local`. Keycloak's cluster discovery opens a
   second transaction while the first holds the write lock, which deadlocks on
   SQLite.
8. **Session purge after its outer transaction.** Every 180 seconds (the
   first time 180 seconds after the start) Keycloak removes expired user
   sessions. It lists the realms in one transaction and deletes in batches,
   each batch in a new transaction. On SQLite the outer transaction already
   holds the write lock, so the first batch waited the whole `busy_timeout`
   (15 seconds) and failed, and every token grant waited with it: sign-in
   answered 503 for those seconds and expired sessions were never removed
   (slice K2). The `sqlite` user-session-persister provider
   (`spi-user-session-persister--provider=sqlite`) runs Keycloak's own purge
   right after the outer transaction commits instead. A purge now takes a
   few milliseconds per realm.

Keycloak logs `KC-SERVICES0047 ... implementing the internal SPI ...` at
build time for `dblock`, `connectionsLiquibase` and `userSessionPersister`.
That is expected: these SPIs are internal to Keycloak.

**Access log.** Both images log every HTTP call as
`[org.keycloak.http.access-log] <method> <path> <status> <milliseconds> ms`.
The path has no query string, and no header, body, cookie or client address
is logged. `identity` cannot log from inside a TACHYON 26.46.02 handler
(TACHYON keeps a handler's stderr only when the handler crashes), so this
log is where a slow or failed Keycloak call shows; `directory` calls show in
the router's log.

## Limits

- **Not supported by the Keycloak project.** The provider uses internal SPIs,
  and the passkey step uses Keycloak's WebAuthn classes (`WebAuthnCredentialProvider`,
  webauthn4j's deprecated `ServerProperty(Set, ...)` constructor, because
  Keycloak's origin check reads that origin set) and counts passkey failures
  in the `password` brute-force category.
  A Keycloak upgrade can break it. Before an upgrade, run the new image on a
  copy of the database file and run `test/run-image-test.sh`.
- **One instance per file.** One process writes the file. Never run two
  containers on the same volume, not even for a moment during a deploy.
- **Local disk only.** WAL mode needs shared memory on the same host. Do not
  put the file on NFS, EFS, SMB, Cloud Storage FUSE or any network file system.
- **Writes are serialized.** All transactions take turns. Fine for a small
  organization; the throughput has not been measured.
- **Weaker schema checks.** Foreign keys are not enforced (cascade deletes
  are). Column types and lengths are not enforced. Keycloak's own code keeps
  the data consistent.
- **Backups.** Stop the container (or snapshot the volume while it is
  stopped) and copy `keycloak.db`, `keycloak.db-wal` and `keycloak.db-shm`
  together.

## Serverless and the cloud

| Platform | SQLite image | Why |
| --- | --- | --- |
| AWS Lambda (container image) | **No** | Keycloak is a long-running JVM (about 1 GB of memory, seconds to start). Lambda runs many short-lived copies with no persistent local disk. |
| Google Cloud Run | **No** | No persistent local disk (only in-memory files, Cloud Storage FUSE or NFS). Scaling and deploys can run two instances at once, which is two writers. |
| ECS on Fargate | **No** | Task storage is lost when the task stops. EFS is NFS. |
| ECS on EC2 with EBS, Fly Machines with a volume, a VM, Kubernetes `StatefulSet` with one replica and a `ReadWriteOnce` volume | **Yes** | One long-running instance with a local block volume. This is ADR 0004's "single-writer service". |

For serverless and cloud deployments, build the PostgreSQL image instead and
use a managed database (for example Aurora Serverless v2, Cloud SQL or Neon).
Then Keycloak can run as its own service with more than one instance:

```bash
docker build --target mesail-keycloak --build-arg KEYCLOAK_DB=postgres -t mesail-keycloak-pg .
```

Run it with `KC_DB_URL=jdbc:postgresql://host:5432/keycloak`,
`KC_DB_USERNAME`, `KC_DB_PASSWORD` and the variables above.

## Native development (no Docker)

```bash
deploy/keycloak/dev/build-native-keycloak.sh ~/Developer/Tools/mesail-keycloak
```

It downloads Keycloak 26.8.2 and the SQLite driver (both checksum-checked),
compiles the provider with the local JDK (21 or newer), and runs
`kc.sh build`. Then start it with the same script the image uses (bash 3.2 is
enough; the supervisor itself needs bash 5.1):

```bash
export KEYCLOAK_HOME=~/Developer/Tools/mesail-keycloak/keycloak
export MESAIL_KEYCLOAK_HOME=$PWD/deploy/keycloak
export MESAIL_KEYCLOAK_DATA=~/Developer/AtlasFylo/mesail/keycloak/dev
export MESAIL_KEYCLOAK_DEV_USERS=true
export MESAIL_IDENTITY_CLIENT_SECRET=dev-identity-secret MESAIL_ACCOUNT_CLIENT_SECRET=dev-account-secret
export MESAIL_KEYCLOAK_DIRECTORY_SECRET=dev-directory-secret
deploy/keycloak/bin/mesail-keycloak-start
```

Keep the SQLite file on a local disk or a local disk image, never on a
network share.

## Testing

`test/run-image-test.sh` needs Docker (on this Mac, Colima: source
`~/.local/share/tachyon-ci/docker-linux/colima-env.zsh` and hold the `colima`
VM lock) and `python3` and `openssl` on the host. It:

1. builds the `mesail-keycloak` stage;
2. starts it on a fresh volume with the one realm, the dev people and a
   stand-in app (`sleep infinity`);
3. runs `test/oidc_check.py` against the realm `mesail`: discovery and issuer
   (and no `mesail-operators`); the password grant for every dev person; the
   RS256 signature against the realm's keys; `iss`, `aud` (only
   `mesail-identity`), `azp`, `typ`, `sid`, roles (`mesail-operator` only for
   the Operator, no old domain roles) and the 60-second lifetime; `ANA` and
   ` Ana ` sign in as `ana`, `ana@example.com` does not; a wrong password;
   introspection (a tampered token is not active); refresh token rotation; no
   browser login offered; no client-credentials grant for `mesail-identity`;
   no `mesail-service` or `mesail-api` client; the service tokens of
   `mesail-account` and `mesail-directory` carry no realm role; the directory
   client creates a person (username given in capitals, stored in
   lowercase) with `mesail.addRequest` and reads it back, refuses a second
   one with the same prefix and six bad usernames, sets `mesail.mustChangePassword` (the claim appears; `account`
   clears it after a password change; only `true` or `false`; the add request
   is in no claim, keeps its value through these changes and refuses 65
   characters), logs out,
   disables (no sign-in) and deletes the person, and gets 403 for clients,
   the `master` realm and creating a realm; `mesail-account` lists and ends
   sessions, resets the password and logs out everywhere; the TOTP endpoint
   (wrong code, short secret, other digits, unknown user, stored, a second
   TOTP refused, the admin event without the secret, a person's own token
   refused), then sign-in without a code, with the confirmation code, one
   step back, one step ahead, the same code again and two steps ahead; the
   directory removes the TOTP; a locked account (other people still sign in)
   and unlocking it; passkeys with the software authenticator: adding one
   (and refusing an unknown person, a wrong or malformed origin, a wrong
   challenge, no user verification, a bad label, the same challenge or
   credential twice, a person's own token), signing in (token claims,
   `lastUsedAt`), refusing a replayed assertion, a wrong signature, origin or
   challenge, no user verification, an unknown or another person's user
   handle, an unknown passkey and no allowed origin, all with the wrong
   password's answer and counted by brute-force detection, a lockout by
   passkey failures (a valid passkey is then refused too), the password still
   working, renaming, an Android origin, 10 passkeys at most, a duplicate
   label, removing one and the directory removing all;
4. restarts the container and runs the checks again (the import is
   skipped);
5. replaces the container, keeping the volume, with the session purge every
   60 seconds; runs the checks again, then signs in as `ana` in a loop until
   a purge finishes and fails when a token grant takes more than 2 seconds,
   when no purge finishes within 150 seconds, or when the purge logs an
   error;
6. ends the stand-in app and checks that the supervisor stops the container;
7. removes the container and the volume.

To run the same checks against a native Keycloak, export
`MESAIL_KEYCLOAK_URL` and the three secrets (plus
`KC_BOOTSTRAP_ADMIN_CLIENT_ID` and `KC_BOOTSTRAP_ADMIN_CLIENT_SECRET` for the
admin event check) and run `python3 deploy/keycloak/test/oidc_check.py`.
