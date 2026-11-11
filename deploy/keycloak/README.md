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
| `realm/mesail-realm.json` | The `mesail` realm: roles, clients, token lifetimes. Imported on the first start. |
| `realm/dev/mesail-users-0.json` | Dev-only test users. Imported only with `MESAIL_KEYCLOAK_DEV_USERS=true`. |
| `bin/mesail-supervisor` | The image entrypoint. Starts Keycloak, waits until it is ready, then starts `MESAIL_APP_COMMAND`. |
| `bin/mesail-keycloak-start` | Checks the environment, prepares the realm import, chooses the database, runs `kc.sh start --optimized`. |
| `bin/mesail-healthcheck` | Docker `HEALTHCHECK`: Keycloak readiness, plus `MESAIL_APP_HEALTH_URL` when set. |
| `test/run-image-test.sh` | Builds the image and proves it works (see [Testing](#testing)). |
| `test/oidc_check.py` | The OIDC checks the test runs (Python standard library and `openssl`). |

## The realm

Realm `mesail`. Issuer: `${MESAIL_PUBLIC_URL}/idp/realms/mesail`.

| Client | Type | Use |
| --- | --- | --- |
| `mesail-web` | public | The Tac frontend. Authorization code flow with PKCE (`S256`) only. Redirect URIs `${MESAIL_PUBLIC_URL}/*`. |
| `mesail-api` | confidential, no flows | The audience of every Mesail access token. The Yon services check `aud` contains `mesail-api`. Its secret is used only for token introspection. |
| `mesail-service` | confidential, client credentials | Service-to-service calls. Tokens carry the role `mesail-service`. |

Realm roles (in the `realm_access.roles` claim): `mesail-member` (every user
gets it by default), `mesail-workspace-admin`, `mesail-admin`,
`mesail-service`. They match `kit/oidc` `RoleMap`.

Tokens: RS256, access token 60 seconds, refresh token rotation on (a refresh
token works once), SSO idle 30 minutes, at most 10 hours. Self-registration and
password reset by email are off. Brute-force protection is on. Passwords need
12 characters.

The realm content is owned by the identity service; this file is the
default the image imports and the test uses. To import other realm files
(for example one realm per mail domain), mount them and set
`MESAIL_KEYCLOAK_IMPORT_DIR`.

The realm file uses `${MESAIL_PUBLIC_URL}`, `${MESAIL_KEYCLOAK_API_SECRET}` and
`${MESAIL_KEYCLOAK_SERVICE_SECRET}` placeholders, which Keycloak fills from the
environment. The import runs only when the realm does not exist yet, so changing
these variables later changes nothing: change the realm in the admin console
instead.

## Running the image

```bash
docker build --target mesail-keycloak -t mesail-keycloak .
```

```bash
docker run -d --name mesail-keycloak -v mesail-keycloak:/data/keycloak \
  -e MESAIL_PUBLIC_URL=https://mail.example.com \
  -e MESAIL_KEYCLOAK_API_SECRET=... -e MESAIL_KEYCLOAK_SERVICE_SECRET=... \
  -e KC_BOOTSTRAP_ADMIN_USERNAME=admin -e KC_BOOTSTRAP_ADMIN_PASSWORD=... \
  -e MESAIL_APP_COMMAND='/path/to/mesail-front' mesail-keycloak
```

| Variable | Required | Meaning |
| --- | --- | --- |
| `MESAIL_PUBLIC_URL` | yes | The public origin, for example `https://mail.example.com`. Keycloak's public URL is this plus `/idp`. |
| `MESAIL_KEYCLOAK_API_SECRET` | with the built-in realm | Secret of `mesail-api` (first start only). |
| `MESAIL_KEYCLOAK_SERVICE_SECRET` | with the built-in realm | Secret of `mesail-service` (first start only). |
| `MESAIL_KEYCLOAK_IMPORT_DIR` | no | A folder of realm files (`*.json`, users in `dev/*.json`) to import instead of the built-in `mesail` realm, for example the identity app's realm templates. Their placeholders come from the environment too. |
| `KC_BOOTSTRAP_ADMIN_USERNAME`, `KC_BOOTSTRAP_ADMIN_PASSWORD` | first start | A temporary admin of the `master` realm. Replace it with a real admin, then remove it. |
| `MESAIL_APP_COMMAND` | no | The app the supervisor starts after Keycloak is ready. Without it the container runs Keycloak only. |
| `MESAIL_APP_HEALTH_URL` | no | `http://127.0.0.1:<port>/<path>` of the app's health route, added to the image health check. |
| `MESAIL_KEYCLOAK_DATA` | no | Folder of the SQLite file. Default `/data/keycloak` (a volume). |
| `MESAIL_KEYCLOAK_DEV_USERS` | no | `true` imports `member@mesail.test` (password `dev-member-password`) and `admin@mesail.test` (`dev-admin-password`). Dev and tests only. |
| `MESAIL_KEYCLOAK_START_TIMEOUT` | no | Seconds the supervisor waits for Keycloak (default 300). |

Inside the container Keycloak listens on `127.0.0.1:8180` under `/idp`, and
its health endpoints on `127.0.0.1:9180/idp/health/*`. The all-in-one front
must proxy `/idp/*` to `http://127.0.0.1:8180/idp/*` with `X-Forwarded-Proto`,
`X-Forwarded-Host` and `X-Forwarded-Port` (Keycloak runs with
`proxy-headers=xforwarded`). To reach Keycloak without a front (tests), set
`KC_HTTP_HOST=0.0.0.0` and publish port 8180.

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

## Testing

`test/run-image-test.sh` needs Docker (on this Mac, Colima: source
`~/.local/share/tachyon-ci/docker-linux/colima-env.zsh` and hold the `colima`
VM lock) and `python3` and `openssl` on the host. It:

1. builds the `mesail-keycloak` stage;
2. starts it on a fresh volume with the dev users and a stand-in app
   (`sleep infinity`);
3. runs `test/oidc_check.py`: discovery and issuer; a PKCE login of
   `member@mesail.test` through the login form; the access token's RS256
   signature against the JWKS; `iss`, `aud`, `azp`, roles and the 60-second
   lifetime; introspection with the `mesail-api` secret (a real token is active,
   a tampered one is not); a `mesail-service` client-credentials token;
4. restarts the container and runs the checks again;
5. replaces the container, keeping the volume, and runs the checks again;
6. ends the stand-in app and checks that the supervisor stops the container;
7. removes the container and the volume.
