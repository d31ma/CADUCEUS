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
| `conf/quarkus.properties` | Turns off Hibernate's database version check (it expects PostgreSQL 14+). |
| `providers/sqlite/` | Java source of `mesail-keycloak-sqlite.jar` (SQLite support), compiled in the `keycloak-providers` stage. Left out of the PostgreSQL image. |
| `providers/mesail/` | Java source of `mesail-keycloak-mesail.jar`: the admin API endpoint that stores a TOTP. In both images. |
| `realm/mesail-realm.json` | The one realm `mesail`: every person and every Operator (ADR 0005). |
| `realm/dev/mesail-users-0.json` | Dev-only people `ana`, `bo`, `eve`, `carol` and `operator`. Imported only with `MESAIL_KEYCLOAK_DEV_USERS=true`. |
| `realm/domain-realm.json`, `realm/mesail-operators-realm.json`, `realm/dev/<domain>-users-0.json` | **Transition only, until Wave 3's switch (L5):** the old realm per mail domain and `mesail-operators` (ADR 0004). Imported only while `MESAIL_KEYCLOAK_DOMAINS` is set. Deleted after L5. |
| `bin/mesail-supervisor` | The image entrypoint. Starts Keycloak, waits until it is ready, runs the provisioning step, then starts `MESAIL_APP_COMMAND`. |
| `bin/mesail-keycloak-start` | Checks the environment, prepares the realm `mesail` (and, in the transition, one realm per domain), chooses the database, runs `kc.sh start --optimized --import-realm`. |
| `bin/mesail-keycloak-provision` | **Transition only:** while the old realms exist, creates the master-realm client `mesail-directory` once, so the old directory service can create realms. Does nothing with the one realm. |
| `dev/build-native-keycloak.sh` | Builds the same Keycloak natively on macOS or Linux (no Docker), for development. |
| `bin/mesail-healthcheck` | Docker `HEALTHCHECK`: Keycloak readiness, plus `MESAIL_APP_HEALTH_URL` when set. |
| `test/run-image-test.sh` | Builds the image and proves it works (see [Testing](#testing)). |
| `test/oidc_check.py` | The checks of the realm `mesail` (Python standard library and `openssl`). |
| `test/legacy_realms_check.py` | **Transition only:** the old checks of the per-domain realms. |
| `test/keycloak_client.py` | HTTP, token, TOTP and admin API helpers both checks use. |

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
| `mesail-identity` | confidential, password grant only | The identity service signs a person in. Keycloak's built-in direct grant flow asks for a TOTP code (`totp` form field) when the person has one set up. Its access token has `aud` `mesail-identity` only (never `mesail-api`, so no service accepts it), the realm role `mesail-operator` for Operators, and the claim `mesail_must_change_password` (boolean, from the user attribute `mesail.mustChangePassword`; absent when the attribute is not set). |
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

**User attributes.** The realm's user profile declares
`mesail.mustChangePassword` (`true` or `false`; only admins see or change it).
Profile verification (`VERIFY_PROFILE`) is off and no field but the username
is required, so a person made through the admin API without an email or a
name can sign in with the password grant.

**Admin API calls** (all tested, in the realm `mesail`, with a
client-credentials token of `mesail-account` or `mesail-directory`):

| Action | Call | Client |
| --- | --- | --- |
| Find a person | `GET /admin/realms/mesail/users?username=<prefix>&exact=true` | both |
| Create a person | `POST /admin/realms/mesail/users` with `username`, `firstName`, `lastName`, `enabled`, `attributes.mesail.mustChangePassword` `["true"]` and a password credential (`temporary: false`) | directory |
| Set or clear must-change | `PUT /admin/realms/mesail/users/<id>` with the user from `GET` and the changed `attributes` | both |
| Disable or enable | `PUT .../users/<id>` with `enabled` | directory |
| Delete | `DELETE .../users/<id>` | directory |
| Set up TOTP with a secret the identity service made, after the person typed a code | `PUT /admin/realms/mesail/mesail/users/<id>/otp` (see below) | account |
| List or remove TOTP | `GET .../users/<id>/credentials`, `DELETE .../users/<id>/credentials/<credential id>` | both |
| Change the password | `PUT .../users/<id>/reset-password` with `{"type":"password","value":"...","temporary":false}` | both |
| List or end sessions | `GET .../users/<id>/sessions`, `DELETE /admin/realms/mesail/sessions/<session id>` | account |
| Log out everywhere | `POST .../users/<id>/logout` | both |
| Lock status, unlock | `GET` / `DELETE /admin/realms/mesail/attack-detection/brute-force/users/<id>` | account |

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
`${MESAIL_ACCOUNT_CLIENT_SECRET}` and `${MESAIL_KEYCLOAK_DIRECTORY_SECRET}`,
filled by Keycloak from the environment. The import runs only when the realm
does not exist yet, so changing a secret later changes nothing: change it
through the admin API.

### Transition until Wave 3's switch (L5)

Until L5 moves every app to Mesail's own tokens, the running apps still use
ADR 0004's realm per domain. While `MESAIL_KEYCLOAK_DOMAINS` is set,
`mesail-keycloak-start` therefore also imports the old realms beside
`mesail`: one realm per listed domain from `realm/domain-realm.json`, the
realm `mesail-operators`, and all files in `realm/dev/` (with
`MESAIL_KEYCLOAK_DEV_USERS=true`). It then needs `MESAIL_SERVICE_SECRET` too,
and `mesail-keycloak-provision` creates the master-realm client
`mesail-directory` (role `create-realm`) for the old directory service. Both
models run side by side in one Keycloak and one SQLite file; they share no
users. `test/legacy_realms_check.py` keeps checking the old realms. After L5,
unset `MESAIL_KEYCLOAK_DOMAINS`; then the old files, the provisioning step and
the legacy check are deleted.

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
| `MESAIL_KEYCLOAK_DIRECTORY_SECRET` | yes | Secret of `mesail-directory` (the directory service's `YON_DIRECTORY_ADMIN_SECRET`). In the transition also the secret of the old master-realm client of the same name. |
| `MESAIL_KEYCLOAK_URL` | no | Keycloak's backend address and the issuer base. Default `http://127.0.0.1:8180/idp`. |
| `MESAIL_KEYCLOAK_DOMAINS` | transition | Mail domains whose old realms are imported beside `mesail`, until L5 (see the transition above). Leave it unset for the one realm. |
| `MESAIL_SERVICE_SECRET` | transition | Secret of `mesail-service` in the old realms. Needed only with `MESAIL_KEYCLOAK_DOMAINS`. |
| `KC_BOOTSTRAP_ADMIN_CLIENT_ID`, `KC_BOOTSTRAP_ADMIN_CLIENT_SECRET` | transition, first start | A temporary admin client of the `master` realm. The provisioning step uses it once to create the old master-realm `mesail-directory`. Not needed for the one realm. Remove it from the environment after the first start. |
| `KC_BOOTSTRAP_ADMIN_USERNAME`, `KC_BOOTSTRAP_ADMIN_PASSWORD` | no | A temporary admin user of the `master` realm, for the admin console (reach it through a tunnel, never the public address). |
| `MESAIL_APP_COMMAND` | no | The app the supervisor starts after Keycloak is ready. Without it the container runs Keycloak only. |
| `MESAIL_APP_HEALTH_URL` | no | `http://127.0.0.1:<port>/<path>` of the app's health route, added to the image health check. |
| `MESAIL_KEYCLOAK_DATA` | no | Folder of the SQLite file. Default `/data/keycloak` (a volume). |
| `MESAIL_KEYCLOAK_IMPORT_DIR` | no | Another folder of realm files laid out like `realm/`. |
| `MESAIL_KEYCLOAK_DEV_USERS` | no | `true` imports the dev people of the realm `mesail`: `ana`, `bo`, `eve`, `carol` (password `dev-<name>-password`) and `operator` (`dev-operator-password`, role `mesail-operator`). Their domains are `directory` data. In the transition it also imports the old realms' dev users. Dev and tests only. |
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

Keycloak logs `KC-SERVICES0047 ... implementing the internal SPI dblock` at
build time. That is expected: the lock and Liquibase SPIs are internal to
Keycloak.

## Limits

- **Not supported by the Keycloak project.** The provider uses internal SPIs.
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
   stand-in app (`sleep infinity`), and checks that provisioning does nothing;
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
   lowercase), refuses a second one with the same prefix and six bad
   usernames, sets `mesail.mustChangePassword` (the claim appears; `account`
   clears it after a password change; only `true` or `false`), logs out,
   disables (no sign-in) and deletes the person, and gets 403 for clients,
   the `master` realm and creating a realm; `mesail-account` lists and ends
   sessions, resets the password and logs out everywhere; the TOTP endpoint
   (wrong code, short secret, other digits, unknown user, stored, a second
   TOTP refused, the admin event without the secret, a person's own token
   refused), then sign-in without a code, with the confirmation code, one
   step back, one step ahead, the same code again and two steps ahead; the
   directory removes the TOTP; a locked account (other people still sign in)
   and unlocking it;
4. restarts the container and runs the checks again (the import is
   skipped);
5. replaces the container, keeping the volume, and runs the checks again;
6. replaces the container with one in transition mode on a new volume
   (`MESAIL_KEYCLOAK_DOMAINS` with three domains and the bootstrap admin
   client), checks that provisioning created the old master-realm client,
   runs `test/oidc_check.py` again and `test/legacy_realms_check.py` (the old
   checks of every domain realm, `mesail-operators` and a realm created
   through the admin API);
7. ends the stand-in app and checks that the supervisor stops the container;
8. removes the container and both volumes.

To run the same checks against a native Keycloak, export
`MESAIL_KEYCLOAK_URL` and the three secrets (plus
`KC_BOOTSTRAP_ADMIN_CLIENT_ID` and `KC_BOOTSTRAP_ADMIN_CLIENT_SECRET` for the
admin event check) and run `python3 deploy/keycloak/test/oidc_check.py`. Add
`CHECK_LEGACY_REALMS=true` when the old realms are imported too.
