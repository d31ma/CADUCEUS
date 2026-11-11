# Mesail on this Mac: native and Floci

There are two ways to run the whole of Mesail on this Mac:

| Script | What runs | Who uses it |
| --- | --- | --- |
| `scripts/native/run-all.sh` | Every Yon app with `ty start` on macOS, the Tac apps as static files, and the router, all native. No VM. | Every slice gate (SLICES.md "Gates"). |
| `scripts/floci/run-all.sh` | The Lambda images in Floci (local AWS), the realtime container, Keycloak and the router, in the dev host or Colima. | The integration and release gates (P1, C1). |

## Native (slice gates)

```sh
MESAIL_OWNER=slice-m3 scripts/native/run-all.sh                          # start all, run every blackbox test, stop
MESAIL_OWNER=slice-m3 MESAIL_TEST_RUN='TestAcMs' scripts/native/run-all.sh
MESAIL_OWNER=slice-m3 scripts/native/run-all.sh messaging directory      # only these Yon apps
MESAIL_OWNER=slice-m3 scripts/native/run-all.sh --up                     # start and keep running
MESAIL_OWNER=slice-m3 scripts/native/run-all.sh --down                   # stop what --up started
```

- It runs as one `caduceus-build` heavy job (`mac-heavy`), unless it already
  runs inside one.
- Each Yon app is copied to `$TMPDIR/mesail-native/<owner>/apps/<app>`
  (your worktree stays clean), given a profile
  `server/application-native.json` (loopback host and, for a FYLO owner, a
  root at `~/Developer/AtlasFylo/mesail/<owner>/native/<service>`, emptied
  at each start), bundled (`ty bundle`, all apps at once) and started with
  `YON_PROFILE=native ty start --port <free port>`. The app's own `.env` is
  copied too (without its datasource lines).
- The Tac apps (`web`, `web-admin`) are bundled with `--target web` and
  served by the router from their `dist/web` folders.
- The router (`deploy/router`, built with `go build`) runs in native mode:
  `/api/v1/<service>/` goes to that app's `ty start` port.
- It prints `export MESAIL_BASE_URL=http://127.0.0.1:<port>` (also in
  `$TMPDIR/mesail-native/<owner>/env`).
- An app that fails to start is a warning: its tests fail, the others run.
- Each app gets a free port, written into its native profile
  (`server.port`), so its own `@Schedule` worker, which calls
  `<app>.localhost:<server.port>`, reaches it.
- Services reach each other through the router. `deploy/lambda/rewrite-config.sh`
  rewrites each copied `server/application.json`: `oidc.issuer` becomes
  `${MESAIL_ISSUER_ORIGIN}/idp/realms/{realm}`; `http://keycloak:8080/idp` and
  `http://127.0.0.1:8180/idp` become the run's Keycloak; `http://<name>.localhost:81xx`
  and `http://127.0.0.1:81xx` keep their host and use the router's port; the
  Keycloak host is added to `outgoing.hosts`. The copied `.env` gets the same
  address change. The Lambda image runs the same script at build time
  (build args `ISSUER_ORIGIN`, `KEYCLOAK_URL`, `SERVICE_BASE`; see the Lambda
  image section for the local forwarders), so every function trusts the same
  issuer and reaches other services through the router.
- Mail: `mail-in` and `mail-out` share the run's blob folder
  (`YON_MAILIN_BLOBS_*`, `mailout.blobfolder`), and `mail-out` reads test DKIM
  keys from a private secrets folder (`mailout.secretfolder`), one RSA 2048 key
  per hosted domain at `dkim/<domain>/<selector>.pem` (selector from the
  directory's `dkimselector`).
- Realtime: when `realtime` runs, the run builds and starts `edge/realtime`
  (public and management ports chosen by the run, the shared
  `MESAIL_REALTIME_GATEWAY_SECRET`), gives `realtime` `YON_REALTIME_GATEWAY_URL`
  and `YON_REALTIME_GATEWAY_SECRET`, and the router sends
  `/api/v1/realtime/socket` and `/stream` to the gateway.
- Scheduled routes: each app's own `@Schedule` worker triggers its relay and
  jobs under `ty start`, every 15 to 60 seconds. So that tests do not wait
  for those intervals, the run also starts `deploy/lambda/scheduler.sh`,
  which calls every route in `stack.json` `schedules` (all nine relay and job
  routes, including `mail-in`'s outbox relay that no worker covers) every
  `nativeScheduleSeconds` (10; `MESAIL_SCHEDULE_SECONDS` overrides it) with a
  `mesail-operators` service token. The extra calls repeat the workers' work,
  and the jobs take leases, so they are harmless.
- An app that declares its own `datasource` in `application.json` (against
  the rule; `search` does today) gets no `.env` datasource; in the copy its
  `fylo:///mnt/data/<service>` becomes this run's root. The Lambda image
  likewise adds no `.env` datasource to such an app.
- When `attachments` runs, the run makes a blob folder in the FYLO test image
  and a random link secret, writes `YON_ATTACHMENTS_ADAPTER=local`,
  `_FOLDER`, `_LINK_BASE` (`<router>/blobs`) and `_LINK_SECRET` into its
  `.env`, and gives the router the same values.
- `MESAIL_ROUTER_PASSTHROUGH` in your environment reaches the router, for the
  realtime gateway until the run starts it itself.
- **Keycloak and tokens (default `MESAIL_KEYCLOAK=native`):**
  - Keycloak 26.8.2 is built once into `~/Developer/Tools/mesail-keycloak`
    (by `deploy/keycloak/dev/build-native-keycloak.sh`, shared by every
    slice) and started for the run on free ports, with its SQLite file in the
    run's FYLO folder. It imports the realms `example.com`, `example.org`,
    `partner.test` and `mesail-operators`, the Keycloak dev users and the
    blackbox users (`ana`, `bo`, `eve`, `carol`, `operator`; see the table
    under "Keycloak, secrets and test users"), with this run's secrets, then
    `mesail-keycloak-provision` creates the `mesail-directory` admin client.
  - The router proxies `/idp/` to it. In each copied app, `oidc.issuer`
    becomes `https://mesail.localhost/idp/realms/{realm}` (the issuer in every
    token) and `http://keycloak:8080/idp` or `http://127.0.0.1:8180/idp`
    becomes this Keycloak.
  - After the apps answer, the run writes the tokens to
    `~/Developer/AtlasFylo/mesail/<owner>/tokens.env` (mode 600; set
    `MESAIL_TOKENS_FILE` for another place; never in a log), adds
    `example.com` and `example.org` to the directory as the operator, and
    calls the directory once as `ana`, `bo` and `eve` so it knows them.
    `carol@partner.test` stays external.
  - Tokens last 60 seconds. The run writes fresh ones just before the tests;
    with `--up`, run `--tokens` before you load the file.
  - Domain verification uses the router's dev DNS responder
    (`MESAIL_ROUTER_DEV_DNS_*`, below), reached as `directory.localhost`.
  - `MESAIL_KEYCLOAK=none` skips all of this; then a `MESAIL_TOKENS_FILE` you
    made yourself is loaded if it exists.
- The FYLO test image must be attached at `~/Developer/AtlasFylo/mesail`;
  the script says how when it is not.
- A whole run with ten apps takes about 25 seconds once warm.
- Logs: `~/Library/Logs/local/CADUCEUS/<owner>/native-<time>/`
  (`app-<app>.log`, `prepare-<app>.log`, `router.log`).

## Floci (integration)

Every Yon app except `realtime` runs as an AWS Lambda container image. On this
Mac the images run in [Floci](https://hub.docker.com/r/floci/floci) 2.4.0, a
local AWS emulator, next to Keycloak and the Mesail router. The same images
later run in Floci on kestrel (through PaaS, like FitNest) and in AWS Lambda.

`realtime` is a long-running container in v1 (ADR 0004, slice R1), not a
Lambda: it holds open WebSockets and event streams. See "Realtime container"
below.

## Quick start

```sh
scripts/floci/run-all.sh                       # build, deploy, run every blackbox test, tear down
MESAIL_TEST_RUN=TestEveryService scripts/floci/run-all.sh
scripts/floci/run-all.sh identity directory    # build and deploy only these functions
```

`run-all.sh` takes the locks itself:

1. It bundles the Tac apps (`web`, `web-admin`) with the pinned `ty` as a
   `caduceus-build` heavy job. A bundle that fails is a warning: the app's
   pages are not served, and the API tests still run.
2. It runs `stack.sh` as one job in the `mesail-docker` root. By default
   (`MESAIL_VM=mesail`) it uses the `mesail` dev host: `mesail-devhost up`
   if it is down, its Docker engine (`mesail-devhost env`) and its Floci on
   `127.0.0.1:14566`, joining Floci's network `mesail_floci_backend`
   (`deploy/floci/compose.devhost.yaml`). With `MESAIL_VM=colima` it takes
   `--vm=colima`, starts Colima and its own Floci instead. `stack.sh` builds
   the images, starts Keycloak, the realtime container and the router, waits
   for `/api/v1/realtime/health` through the router, deploys the functions,
   runs `go test -tags blackbox` in `tests/blackbox`, removes
   what it started, and takes the dev host down or stops Colima only if it
   started it. In the dev host it only stops its own compose containers
   (`compose stop`; the next run starts them again) and deletes, through the
   Lambda API, exactly the functions named in `stack.json`; it never removes
   any container, volume or network there. Only with `MESAIL_VM=colima`, where it started Floci itself,
   does it also remove Floci's function containers, EFS volumes and network.
   No script ever removes the realtime data volume `mesail-realtime-data`. An image that fails to build is a warning: that function is
   not deployed and its tests fail.

Logs: `~/Library/Logs/local/CADUCEUS/<MESAIL_OWNER>/floci-<time>/`, one file
per step. Set `MESAIL_OWNER` to your slice name (for example `slice-m3`).

Settings (environment, or a `.env` file copied from `.env.example`):

| Name | Meaning |
| --- | --- |
| `MESAIL_OWNER` | Lock owner and log folder (default `floci-run-all`). |
| `MESAIL_TEST_RUN` | `go test -run` filter for `tests/blackbox`. |
| `MESAIL_SKIP_STATIC=1` | Skip the Tac bundles and the static upload. |
| `MESAIL_SKIP_BUILD=1` | Reuse the images that are already built. |
| `MESAIL_VM` | `mesail` (default, the dev host) or `colima`. |

## The pieces

| Path | What it is |
| --- | --- |
| `Dockerfile` target `lambda` | One image per Yon app: `--build-arg APP=<service> --build-arg FYLO=<true/false>`. |
| `Dockerfile` target `realtime` | The realtime container: the `realtime` Yon app and the `edge/realtime` gateway (helper stage `realtime-gateway-build`). |
| `Dockerfile` target `router` | The public origin (`deploy/router`). |
| `deploy/lambda/` | Image helpers: tool install with checksums, the Lambda profile, the Lambda start script and the realtime start script (`realtime-start.sh`). |
| `deploy/floci/stack.json` | The Floci description: functions, memory, timeouts, FYLO owners, containers (`"kind": "container"`), file system, bucket, realms and test users. |
| `deploy/floci/keycloak-users/` | The blackbox test users, added to the Keycloak image by the target `keycloak-blackbox`. |
| `docker-compose.yml` | Floci, Keycloak, the realtime container and the router on the network `mesail_floci_backend`. |
| `scripts/floci/*.sh` | `build`, `up`, `deploy`, `tokens`, `test`, `down`, `stack`, `run-all`. |

### Lambda image (`Dockerfile` target `lambda`)

- Ubuntu 24.04, because the FYLO Linux binary needs glibc 2.39.
- Go 1.27.3, `ty` 26.46.02 and `fylo` 26.45.07, each checked against its
  published SHA-256. The Go toolchain stays in the image because `ty start`
  builds handlers with it.
- The AWS Lambda Web Adapter 0.9.1 as a Lambda extension
  (`/opt/extensions/lambda-adapter`). It forwards each invocation to
  `ty start` on `127.0.0.1:8080` and waits for
  `/api/v1/<service>/health` before the first request.
- The app is bundled during the build at `/tmp/mesail-app` and stored in
  `/opt/mesail`. Lambda's filesystem is read-only outside `/tmp`, and
  `ty start` writes beside the app, so the start script copies the app and
  its handler cache back to the same `/tmp` paths at cold start (16 MB each).
- `YON_PROFILE=lambda` selects `server/application-lambda.json`, which the
  build writes into the image only (loopback host). A FYLO owner's datasource
  is in the app's `.env`, written by the build:
  `YON_DATASOURCE_DRIVER=fylo` and `YON_DATASOURCE_URL=fylo:///mnt/data/<service>`
  (apps keep no datasource in `application.json`).
- `ty` reads only the app folder's `.env`, not the process environment. So at
  cold start the start script appends the function's own `YON_*`
  environment variables (its secrets, for example
  `YON_SERVICE_CLIENT_SECRET`) to that `.env`, leaving out `ty`'s runtime
  variables (`YON_PROFILE`, `YON_EGRESS_ENFORCEMENT`, `YON_ISOLATION`,
  `YON_PORT`, `YON_HOST`). Set secrets as Lambda environment variables (or
  from a secret manager), never in the image.
- `YON_EGRESS_ENFORCEMENT=proxy`: Lambda cannot confine processes (TACHYON
  ADR 0113).
- **Local forwarders for private addresses.** TACHYON's outgoing request
  guard drops private (RFC 1918) addresses when it resolves a name, and in
  Docker `keycloak` and `router` resolve to such addresses, so a handler could
  not reach them. A listed localhost name is the allowed way: the guard
  connects it to 127.0.0.1. So the image starts `socat` forwarders
  (`deploy/lambda/forwards.sh`, `MESAIL_FORWARDS`, default
  `18180=keycloak:8080,18080=router:8080`) before `ty start`, and the build
  rewrites the config to `http://keycloak.localhost:18180/idp` and
  `http://router.localhost:18080` (build args `KEYCLOAK_URL`, `SERVICE_BASE`,
  `FORWARDS`). Where Keycloak and the API have public names (AWS), build with
  those names and an empty `FORWARDS`.
- The start script creates `/mnt/data/<service>`, hands it to the user
  `mesail` (uid 10001) and drops root before `ty start` when it runs as root
  (Floci). In AWS, Lambda already runs as a non-root user and the EFS access
  point (`PosixUser` 10001, root directory `/<service>`) owns the folder.

### Realtime container (`Dockerfile` target `realtime`)

- One image, `mesail-realtime:<tag>`, built from the same Ubuntu base and
  tools as the Lambda images. `stack.json` marks `realtime` with
  `"kind": "container"` and `"image": "mesail-realtime"`, so `build.sh`
  builds the Dockerfile target named after the service, and `deploy.sh` and
  `down.sh` make or delete no function for it.
- One container runs two processes, started by
  `deploy/lambda/realtime-start.sh` as the user `mesail` (uid 10001):
  1. `ty start` of `apps/realtime` on `0.0.0.0:8106`
     (`--allow-non-loopback`), bundled during the build at
     `/opt/mesail/realtime`.
  2. The gateway `edge/realtime` (built with `go build` in the helper stage
     `realtime-gateway-build`): public port `0.0.0.0:8116`, management port
     `127.0.0.1:8117`, backend `http://127.0.0.1:8106`. The start script
     always sets the management port and the backend itself, so the
     management port never leaves the container.
- At start the script writes the app's `.env` (mode 600, never printed):
  `YON_DATASOURCE_DRIVER=fylo`, `YON_DATASOURCE_URL=fylo:///data/realtime`,
  `YON_REALTIME_GATEWAY_URL=http://127.0.0.1:8117`,
  `YON_REALTIME_GATEWAY_SECRET` (copied from
  `MESAIL_REALTIME_GATEWAY_SECRET`, so both sides always hold the same value)
  and every other `YON_*` variable of the container (for example
  `YON_SERVICE_CLIENT_SECRET`). It refuses to start without
  `MESAIL_REALTIME_GATEWAY_SECRET`.
- It passes `SIGTERM` to both processes, and stops the container when either
  process ends. Compose runs it with `init: true` and a 40 s stop grace
  period (the gateway closes every connection with 1001 first).
- **Host name:** the router's proxy sends the target's host
  (`realtime:8106`) as `Host`. Tachyon answers only localhost names, IP
  addresses and the names in `server.hosts` (else 403 TY1366). The image
  therefore has a profile `server/application-container.json`
  (`YON_PROFILE=container`, image only) with `"hosts": ["realtime"]`. If the
  container gets another network name, change that list.
- `YON_EGRESS_ENFORCEMENT=proxy`, as in the Lambda images.
- The FYLO root is on the named volume `mesail-realtime-data`, mounted at
  `/data`. Our scripts never remove it, so data stays between runs (in the
  dev host, Docker volumes live on the ATLAS disk).
- No published ports: the router reaches it on the network `backend`.
- Compose settings: `MESAIL_REALTIME_GATEWAY_SECRET` (generated for each
  run), `MESAIL_REALTIME_ORIGINS` = `MESAIL_PUBLIC_URL`, and
  `YON_SERVICE_CLIENT_SECRET` = `MESAIL_SERVICE_SECRET`.

### Functions (`deploy.sh`)

- One function `mesail-<service>` per Yon app (not for a `"kind":
  "container"` entry such as `realtime`), `PackageType=Image`, arm64,
  memory and timeout from `stack.json`.
- FYLO owners get one EFS access point each on the file system
  `mesail-data`, mounted at `/mnt/data`, a VPC configuration (Floci needs
  one; the IDs are placeholders), and **reserved concurrency 1**, so only one
  warm instance writes the root (ADR 0004 section 4).
- In Floci the access point's root directory and user are not applied: each
  access point is its own Docker volume `floci-aws-efs-<id>-<hash>`.

### Router (`deploy/router`)

One HTTP origin, like FitNest's gateway:

| Path | Goes to |
| --- | --- |
| `/_router/ready` | The router itself. |
| `/api/v1/realtime/gateway/...` (and any `MESAIL_ROUTER_BLOCKED` prefix) | Always 404: the realtime gateway's private routes are never public. |
| `MESAIL_ROUTER_PASSTHROUGH` prefixes | Proxied as they are, with streaming and WebSocket upgrades. When several prefixes match, the longest one wins. They are checked before `/api/v1/<service>/`, so a service reached this way needs no entry in `MESAIL_ROUTER_SERVICES`. In `docker-compose.yml`: `/api/v1/realtime/socket` and `/api/v1/realtime/stream` go to the gateway (`http://realtime:8116`), and every other `/api/v1/realtime/` path goes to the realtime Yon app (`http://realtime:8106`). The gateway's management port 8117 is never routed. |
| `/blobs/<domain>/<name>?<signature>` | `kit/blobs` `LocalHandler` (GET and PUT only) when `MESAIL_ROUTER_BLOBS_FOLDER`, `_BASE` and `_SECRET` (32+ characters) are set: the signed upload and download links of the local blob store. The values match the attachments app's `YON_ATTACHMENTS_FOLDER`, `_LINK_BASE` and `_LINK_SECRET`. Without them, 404. |
| `/api/v1/<service>/...` | Lambda `mesail-<service>`, invoked with an HTTP API v2.0 event. The event's `host` header is `localhost` (Tachyon answers only local names) and the original host is in `x-forwarded-host`. Unknown service: 404. Body over 6 MiB: 413. A function error: 502. |
| `/idp/...` | Keycloak (served under `/idp`), with the original `Host`. |
| `GET /_dev/dns-query` | Only when `MESAIL_ROUTER_DEV_DNS_DOMAINS` is set (native runs): a DNS-over-HTTPS (JSON) responder for those test domains, so the directory's domain check can pass locally: MX `10 <MESAIL_ROUTER_DEV_DNS_MAIL_HOST>.`, TXT `v=spf1 <MESAIL_ROUTER_DEV_DNS_SPF> -all`, any `<selector>._domainkey` a placeholder DKIM key, `_dmarc` `v=DMARC1; p=reject`; anything else NXDOMAIN. Otherwise 404. |
| `/admin`, `/admin/...` | The `web-admin` bundle in the bucket `mesail-static` under `admin/`. |
| everything else | The `web` bundle under `web/`. A missing page (no file extension, or a browser asking for HTML) gets the app's `index.html`. |

Limits at the router:

- **Per-address rate limit** (`MESAIL_ROUTER_RATE_LIMIT`, default 10 per
  minute per client address) on `POST /api/v1/identity/session` and
  `POST /api/v1/identity/session/refresh`
  (`MESAIL_ROUTER_LIMITED_ROUTES`). Over the limit: 429 with `Retry-After`.
  Identity keeps no attempt counter; Keycloak's brute-force detection
  covers each account. In AWS the same rule is an AWS WAF rate-based rule on
  the API Gateway stage (API Gateway throttling is not per address).
- **One writer per FYLO root:** in Lambda mode the router sends one request
  at a time to each FYLO-owning function (`MESAIL_ROUTER_SINGLE_WRITERS`,
  filled from `stack.json`) and queues the others, so reserved concurrency 1
  never answers 429 to a user. Other functions take up to 1000 at once.

Static replies carry `Content-Security-Policy`, `X-Content-Type-Options`,
`X-Frame-Options` and `Referrer-Policy`.

In AWS the same routes become an API Gateway HTTP API (Lambda proxy
integrations, payload 2.0) plus S3 and CloudFront for the static files.

### Keycloak, secrets and test users

Keycloak is the `mesail-keycloak` image (Keycloak 26.8.2 on SQLite, owned by
`deploy/keycloak`), extended by the test-only target `keycloak-blackbox`,
which adds the blackbox users from `deploy/floci/keycloak-users/`. It runs
with `MESAIL_KEYCLOAK_DOMAINS=example.com,example.org,partner.test`, the dev
users on, its SQLite file on a tmpfs (no volume), and listens on port 8080
under `/idp` inside the network. Its hostname is
`${MESAIL_ISSUER_ORIGIN}/idp` (default `https://mesail.localhost/idp`), so
every token's issuer is `https://mesail.localhost/idp/realms/<domain>`, the
value the apps put in their `oidc.issuer`; Lambdas fetch keys from
`http://keycloak:8080/idp/realms/<domain>/protocol/openid-connect/certs`.

**Secrets.** Each run makes one set of random client secrets in a private
temporary file (mode 600, removed at the end, never in a log), or uses the
file named by `MESAIL_SECRETS_FILE`:

| Generated | Keycloak reads | Services read (only the apps that use the name) |
| --- | --- | --- |
| `MESAIL_IDENTITY_CLIENT_SECRET` | same name | `YON_IDENTITY_CLIENT_SECRET`, `YON_DIRECTORY_REALMSECRETS_IDENTITY` |
| `MESAIL_SERVICE_SECRET` | same name | `YON_SERVICE_CLIENT_SECRET`, `YON_DIRECTORY_REALMSECRETS_SERVICE` |
| `MESAIL_ACCOUNT_CLIENT_SECRET` | same name | `YON_IDENTITY_ACCOUNT_SECRET`, `YON_DIRECTORY_REALMSECRETS_ACCOUNT` |
| `MESAIL_KEYCLOAK_DIRECTORY_SECRET` | same name | `YON_DIRECTORY_ADMIN_SECRET` |
| `KC_BOOTSTRAP_ADMIN_CLIENT_SECRET` | same name (first start: creates `mesail-directory`) | none |
| `MESAIL_LINK_SECRET` | none | `YON_ATTACHMENTS_LINK_SECRET` (and the router's `/blobs`) |
| `MESAIL_REALTIME_GATEWAY_SECRET` | none | The realtime container: the gateway, and the Yon app as `YON_REALTIME_GATEWAY_SECRET` |

The map is `secrets` in `stack.json`. A service gets a name when its app
folder mentions it (in `.env.example`, code or `application.json`). In Floci
the values are the functions' environment variables, which the start script
writes into the app's `.env` at cold start; natively they go straight into
the copied `.env` (an app's own `.env` value wins).

**Test users** (tokens from the confidential client `mesail-identity`,
password grant, by `tokens.sh`):

| Token | User | Realm | Roles |
| --- | --- | --- | --- |
| `MESAIL_TOKEN_ANA` | `ana@example.com` | `example.com` | `mesail-member`, `mesail-admin` |
| `MESAIL_TOKEN_BO` | `bo@example.com` | `example.com` | `mesail-member` |
| `MESAIL_TOKEN_EVE` | `eve@example.org` | `example.org` | `mesail-member` |
| `MESAIL_TOKEN_CAROL` | `carol@partner.test` | `partner.test` | `mesail-member` |
| `MESAIL_TOKEN_OPERATOR` | `operator@mesail.test` | `mesail-operators` | `mesail-operator` |

The Keycloak image's own dev users (`member@<domain>`, `admin@<domain>`)
exist too. Passwords are in `stack.json` (dev only).

### Scheduler (Floci)

In Lambda form no `@Schedule` worker runs, so the `scheduler` container
(`Dockerfile` target `scheduler`, `deploy/lambda/scheduler.sh` on a pinned
curl image) calls every route in `stack.json` `schedules` through the router
every `scheduleSeconds` (60) with a `mesail-operators` service token from
`mesail-service` (`MESAIL_SERVICE_SECRET`). It logs only the path and the
status code. In AWS this becomes EventBridge Scheduler rules with a small
token-fetching trigger.

## Ports on the Mac (127.0.0.1 only)

| Port | Service |
| --- | --- |
| 18080 | Router (`MESAIL_BASE_URL` of the blackbox tests) |
| 18180 | Keycloak under `/idp` |
| 14566 / 4566 | Floci (AWS API) in the dev host / in Colima |

## Not done yet

- Native app targets of the Tac apps (macOS, Windows, Linux, iOS, Android).
- EventBridge schedules that call each service's `/outbox/relay` and
  `/jobs/<job>` routes in Lambda form.
- The SMTP edge container (`edge/smtp`, slice M1) and `MESAIL_SMTP_ADDR`.
- Event relay schedules (EventBridge) and the realtime WebSocket API.
- Deployment to kestrel through PaaS.
