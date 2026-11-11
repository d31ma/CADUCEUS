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
| `providers/sqlite/` | Java source of `mesail-keycloak-sqlite.jar`, compiled in the `keycloak-providers` stage. |
| `realm/domain-realm.json` | The template of one mail domain's realm (`${MESAIL_REALM}` = the domain). |
| `realm/mesail-operators-realm.json` | The `mesail-operators` realm, for the people who run the installation. |
| `realm/dev/<realm>-users-0.json` | Dev-only test users for `example.com`, `example.org`, `partner.test` and `mesail-operators`. Imported only with `MESAIL_KEYCLOAK_DEV_USERS=true`. |
| `bin/mesail-supervisor` | The image entrypoint. Starts Keycloak, waits until it is ready, runs the provisioning step, then starts `MESAIL_APP_COMMAND`. |
| `bin/mesail-keycloak-start` | Checks the environment, renders one realm per domain, chooses the database, runs `kc.sh start --optimized --import-realm`. |
| `bin/mesail-keycloak-provision` | Creates the master-realm client `mesail-directory` once, so the directory service can create realms through the admin API. |
| `dev/build-native-keycloak.sh` | Builds the same Keycloak natively on macOS or Linux (no Docker), for development. |
| `bin/mesail-healthcheck` | Docker `HEALTHCHECK`: Keycloak readiness, plus `MESAIL_APP_HEALTH_URL` when set. |
| `test/run-image-test.sh` | Builds the image and proves it works (see [Testing](#testing)). |
| `test/oidc_check.py` | The OIDC checks the test runs (Python standard library and `openssl`). |

## Realms: one per mail domain

Each hosted mail domain has its own realm, named after the domain (for
example `example.com`). The realm keeps that domain's users apart: a token
from one realm is never valid in another, and its issuer names the domain.
People who run the installation sign in to the separate realm
`mesail-operators`.

**Keycloak is reached only from the backend.** No browser talks to it, and
`/idp` is not routed from the public address. The identity service signs
users in server to server.

| Client | In | Type | Use |
| --- | --- | --- | --- |
| `mesail-identity` | every realm | confidential, password grant only | The identity service signs a user in. Keycloak's built-in direct grant flow asks for a TOTP code (`totp` form field) when the user has one set up. Brute-force detection locks an account for a while after 10 failures. |
| `mesail-service` | every realm | confidential, client credentials | Service-to-service calls. Tokens carry only the role `mesail-service`. A call about one domain uses that domain's realm; install-wide calls use `mesail-operators`. |
| `mesail-account` | domain realms | confidential, client credentials; service account with `realm-management` `view-users` and `manage-users` only | The identity service changes a user's own settings through this realm's admin API: password reset, TOTP set and remove, logout everywhere, lock status and unlock. It gets 403 on anything else and on other realms. |
| `mesail-api` | every realm | audience only | Every access token's `aud` contains `mesail-api`. |
| `mesail-directory` | `master` | confidential, client credentials, role `create-realm` | The directory service creates a new domain's realm through the admin API. Created by `mesail-keycloak-provision`. Keycloak makes the creator an admin of each realm it creates. |

There is no browser client. Keycloak's own `account-console` and
`security-admin-console` clients still exist in each realm (Keycloak always
creates them), but nothing routes browsers to them.

Realm roles (in `realm_access.roles`): `mesail-member` (every user of a domain
gets it), `mesail-admin` (admin of that domain), `mesail-service` (service
accounts only; they get no default role), and in `mesail-operators` the role
`mesail-operator` (given to each operator explicitly; that realm's default role
is empty).

**Account settings through the admin API** (all tested, with a
`mesail-account` client-credentials token of the user's realm):

| Action | Call |
| --- | --- |
| Find the user | `GET /admin/realms/<domain>/users?username=<address>&exact=true` |
| Set up TOTP with a secret the identity service made | `PUT /admin/realms/<domain>/users/<id>` with `{"credentials":[{"type":"otp","userLabel":"Mesail","secretData":"{\"value\":\"<base32>\"}","credentialData":"{\"subType\":\"totp\",\"digits\":6,\"period\":30,\"algorithm\":\"HmacSHA1\",\"counter\":0,\"secretEncoding\":\"BASE32\"}"}]}` |
| List or remove TOTP | `GET .../users/<id>/credentials`, `DELETE .../users/<id>/credentials/<credential id>` |
| Change the password | `PUT .../users/<id>/reset-password` with `{"type":"password","value":"...","temporary":false}` |
| Log out everywhere | `POST .../users/<id>/logout` |
| Lock status, unlock | `GET` / `DELETE /admin/realms/<domain>/attack-detection/brute-force/users/<id>` |

**Sign-in answers.** A wrong password, a missing or wrong TOTP code and a
locked account all get the same answer: HTTP 400 `invalid_grant`,
"Invalid user credentials". Keycloak does not say which (so nobody can probe
accounts). The identity service can ask for the lock status with
`mesail-account`. Lockout: 10 failures lock the account for up to 15 minutes,
and two failures within 1 second lock it for 60 seconds (Keycloak's quick login
check).

Tokens: RS256, access token 60 seconds, refresh token rotation on (a refresh
token works once), SSO idle 30 minutes, at most 10 hours. Passwords need 12
characters. Self-registration and password reset by email are off.

**Issuer:** `${MESAIL_KEYCLOAK_URL}/realms/<domain>`. `MESAIL_KEYCLOAK_URL`
is the backend address of Keycloak, default `http://127.0.0.1:8180/idp` (the
all-in-one image). If Keycloak runs as its own host, use that host, for
example `http://keycloak:8180/idp`. Keys:
`${MESAIL_KEYCLOAK_URL}/realms/<domain>/protocol/openid-connect/certs`.

**Adding a domain.** Two ways, both tested on SQLite:
- at run time: the directory service takes a client-credentials token from
  the `master` realm with `mesail-directory`, renders `domain-realm.json`
  itself (the admin API does not fill `${...}` placeholders), and posts it to
  `${MESAIL_KEYCLOAK_URL}/admin/realms`; then it creates users with
  `POST /admin/realms/<domain>/users`;
- at start: add the domain to `MESAIL_KEYCLOAK_DOMAINS`. A realm that exists
  already is skipped, so restarts change nothing.

The realm templates use the placeholders `${MESAIL_REALM}` (filled by
`mesail-keycloak-start` for each domain), `${MESAIL_IDENTITY_CLIENT_SECRET}`,
`${MESAIL_SERVICE_SECRET}` and `${MESAIL_ACCOUNT_CLIENT_SECRET}` (filled by
Keycloak from the environment; the directory service fills them itself when it
creates a realm). The
import runs only for realms that do not exist yet, so changing a secret later
changes nothing: change it through the admin API.

## Running the image

```bash
docker build --target mesail-keycloak -t mesail-keycloak .
```

```bash
docker run -d --name mesail-keycloak -v mesail-keycloak:/data/keycloak \
  -e MESAIL_KEYCLOAK_DOMAINS=example.com,example.org \
  -e MESAIL_IDENTITY_CLIENT_SECRET=... -e MESAIL_SERVICE_SECRET=... \
  -e MESAIL_KEYCLOAK_DIRECTORY_SECRET=... \
  -e KC_BOOTSTRAP_ADMIN_CLIENT_ID=bootstrap-admin -e KC_BOOTSTRAP_ADMIN_CLIENT_SECRET=... \
  -e MESAIL_APP_COMMAND='/path/to/mesail-front' mesail-keycloak
```

| Variable | Required | Meaning |
| --- | --- | --- |
| `MESAIL_IDENTITY_CLIENT_SECRET` | yes | Secret of `mesail-identity`, the same in every realm (the identity service's `YON_IDENTITY_CLIENT_SECRET`). Used when a realm is imported. |
| `MESAIL_SERVICE_SECRET` | yes | Secret of `mesail-service` in every imported realm (the services' `YON_SERVICE_CLIENT_SECRET`). |
| `MESAIL_ACCOUNT_CLIENT_SECRET` | yes | Secret of `mesail-account` in every imported domain realm (the identity service's `YON_IDENTITY_ACCOUNT_SECRET`). |
| `MESAIL_KEYCLOAK_DOMAINS` | no | Mail domains whose realms are imported at start, separated by commas or spaces. Existing realms are skipped. |
| `MESAIL_KEYCLOAK_URL` | no | Keycloak's backend address and the issuer base. Default `http://127.0.0.1:8180/idp`. |
| `MESAIL_KEYCLOAK_DIRECTORY_SECRET` | no | Turns on the `mesail-directory` admin client and is its secret (the directory service's `YON_DIRECTORY_ADMIN_SECRET`). |
| `KC_BOOTSTRAP_ADMIN_CLIENT_ID`, `KC_BOOTSTRAP_ADMIN_CLIENT_SECRET` | first start | A temporary admin client of the `master` realm. The provisioning step uses it once to create `mesail-directory`. Remove it from the environment after the first start. |
| `KC_BOOTSTRAP_ADMIN_USERNAME`, `KC_BOOTSTRAP_ADMIN_PASSWORD` | no | A temporary admin user of the `master` realm, for the admin console (reach it through a tunnel, never the public address). |
| `MESAIL_APP_COMMAND` | no | The app the supervisor starts after Keycloak is ready. Without it the container runs Keycloak only. |
| `MESAIL_APP_HEALTH_URL` | no | `http://127.0.0.1:<port>/<path>` of the app's health route, added to the image health check. |
| `MESAIL_KEYCLOAK_DATA` | no | Folder of the SQLite file. Default `/data/keycloak` (a volume). |
| `MESAIL_KEYCLOAK_IMPORT_DIR` | no | Another folder of realm files laid out like `realm/`. |
| `MESAIL_KEYCLOAK_DEV_USERS` | no | `true` imports the dev users: `member@<domain>` (password `dev-member-password`) and `admin@<domain>` (`dev-admin-password`) in each dev domain, and `operator@mesail.test` (`dev-operator-password`). Dev and tests only. |
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
export MESAIL_KEYCLOAK_DOMAINS=example.com,example.org,partner.test MESAIL_KEYCLOAK_DEV_USERS=true
export MESAIL_IDENTITY_CLIENT_SECRET=dev-identity-secret MESAIL_SERVICE_SECRET=dev-service-secret
deploy/keycloak/bin/mesail-keycloak-start
```

Keep the SQLite file on a local disk or a local disk image, never on a
network share.

## Testing

`test/run-image-test.sh` needs Docker (on this Mac, Colima: source
`~/.local/share/tachyon-ci/docker-linux/colima-env.zsh` and hold the `colima`
VM lock) and `python3` and `openssl` on the host. It:

1. builds the `mesail-keycloak` stage;
2. starts it on a fresh volume with three domain realms, the dev users, the
   bootstrap admin client and a stand-in app (`sleep infinity`);
3. runs `test/oidc_check.py` against every domain realm and
   `mesail-operators`: discovery and issuer; the password grant through
   `mesail-identity`; the RS256 signature against the realm's keys; `iss`,
   `aud`, `azp`, `typ`, roles and the 60-second lifetime; a wrong password;
   refresh token rotation; introspection (a tampered token is not active, and
   a token of one domain is not active in another); a `mesail-service` token
   without member or operator roles; no browser login offered; every
   `mesail-account` action above, and its 403s; a locked account and unlocking it; then it creates the realm `added.test` through
   the admin API with `mesail-directory`, adds a user and signs in to it;
4. restarts the container and runs the checks again;
5. replaces the container, keeping the volume, without the bootstrap admin
   client, and runs the checks again;
6. ends the stand-in app and checks that the supervisor stops the container;
7. removes the container and the volume.
