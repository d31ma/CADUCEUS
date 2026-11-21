# Mesail on this Mac: native and Floci

There are two ways to run the whole of Mesail on this Mac:

| Script | What runs | Who uses it |
| --- | --- | --- |
| `scripts/native/run-all.sh` | Every Yon app with `ty start` on macOS, the Tac apps as static files, and the router, all native. No VM. | Every slice gate (SLICES.md "Gates"). |
| `scripts/floci/run-all.sh` | The Lambda images in Floci, the realtime container, Keycloak and the router, in the dev host or Colima. | The integration and release gates (P1, C1). |

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
- Services reach each other through the router. `deploy/lambda/rewrite-config.sh
  <application.json> <keycloak url> <service base> [dev dns resolver]`
  rewrites each copied `server/application.json`: `http://keycloak:8080/idp` and
  `http://127.0.0.1:8180/idp` become the run's Keycloak; `http://<name>.localhost:81xx`
  and `http://127.0.0.1:81xx` keep their host and use the router's port; the
  Keycloak host is added to `outgoing.hosts`. So `oidc.issuer`, `oidc.jwks`,
  `oidc.serviceTokens` and `identity.issuer` all become
  `<router>/api/v1/identity...` in every app: every app trusts the tokens
  that `identity` signs. The copied `.env` gets the same address change. The
  Lambda image runs the same script at build time (build args `KEYCLOAK_URL`,
  `SERVICE_BASE`; see the Lambda image section for the local forwarders).
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
  which calls every route in `stack.json` `schedules` (all ten relay and job
  routes, including `mail-in`'s outbox relay that no worker covers and
  directory's `/jobs/removals`) every `nativeScheduleSeconds` (10;
  `MESAIL_SCHEDULE_SECONDS` overrides it) with an install-wide service token
  from `identity` (`POST /api/v1/identity/service-token`, domain `*`). The
  extra calls repeat the workers' work, and the jobs take leases, so they are
  harmless.
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
    run's FYLO folder. It imports the one realm `mesail` and its dev people
    (`MESAIL_KEYCLOAK_DEV_USERS=true`; see "People" below), with this run's
    secrets. Its issuer is `http://127.0.0.1:<its port>/idp/realms/mesail`,
    the same URL `identity` uses to reach it.
  - Only the services talk to Keycloak. The router has no `/idp/` route in
    this stack. In each copied app, `http://keycloak:8080/idp` or
    `http://127.0.0.1:8180/idp` becomes this Keycloak.
  - After the apps answer, the run seeds the directory (see "Seeding the
    directory" below), then writes the tokens to
    `~/Developer/AtlasFylo/mesail/<owner>/tokens.env` (mode 600; set
    `MESAIL_TOKENS_FILE` for another place; never in a log).
  - The directory gets `YON_DIRECTORY_DEVELOPMENTSEED=true` in its copied
    `.env` (`stack.json` `appSettings`; it wins over the app's own `.env`).
    It turns on `POST /api/v1/directory/people/seed`. Never set it in
    production.
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

Every Yon app except `realtime` runs as a Lambda container image. On this
Mac the images run in [Floci](https://hub.docker.com/r/floci/floci) 2.4.0, an
emulator of the Lambda, S3 and EFS APIs, next to Keycloak and the Mesail
router. The same images later run in Floci on kestrel (through PaaS, like
FitNest). Mesail runs on-premise only (ADR 0007).

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
| `MESAIL_WARMUP_SECONDS=<n>` | Before the tests, repeat the directory seed and one sign-in (`ana`) every 15 s until both pass, for up to `<n>` seconds (300 for slice K6's runs). A first FYLO call after a cold start can pass the 5 s store deadline until slice K10; a failure here is that known class, not a test result. Unset: seed once. |
| `MESAIL_VM` | `mesail` (default, the dev host) or `colima`. |
| `MESAIL_ROUTER_RATE_LIMIT` | The router's sign-ins per minute per address. The Floci scripts set 3000 (`stack.json` `routerRateLimit`) when it is not set: the blackbox suite can send over 1000 sign-ins in one minute (slice K20: 1068, 30 router 429s at 1000). The `docker-compose.yml` default (1000), the native runner (1000) and production (10, OPERATIONS.md) do not change. |
| `MESAIL_SMTP_PORT` | The Mac port of the SMTP edge (default 12525, `stack.json` `smtp.port`). |
| `MESAIL_FYLO_TRACE=1` | Build the test images with the fylo trace wrapper and the start snapshot (see "FYLO trace and capture before retry"). Off by default; never for kestrel or production images. |

## The pieces

| Path | What it is |
| --- | --- |
| `Dockerfile` target `lambda` | One image per Yon app: `--build-arg APP=<service> --build-arg FYLO=<true/false>`. |
| `Dockerfile` target `realtime` | The realtime container: the `realtime` Yon app and the `edge/realtime` gateway (helper stage `realtime-gateway-build`). |
| `Dockerfile` target `router` | The public origin (`deploy/router`). |
| `deploy/lambda/` | Image helpers: tool install with checksums, the Lambda profile, the Lambda start script and the realtime start script (`realtime-start.sh`). |
| `deploy/floci/stack.json` | The Floci description: functions, memory, timeouts, FYLO owners, containers (`"kind": "container"`), file system, bucket, Keycloak realm and image, secrets, dev settings (`appSettings`), people, domains and schedules. |
| `docker-compose.yml` | Floci, Keycloak, the realtime container, the router and the two schedulers (`scheduler`, `scheduler-daily`) on the network `mesail_floci_backend`. |
| `deploy/floci/compose.blobs.yaml` | Used by `blobs.sh` only: makes the `blobs` folder in the shared EFS volume and mounts only that folder into the router, with `MESAIL_ROUTER_BLOBS_*`. |
| `scripts/floci/*.sh` | `build`, `up`, `deploy`, `blobs`, `tokens`, `test`, `down`, `stack`, `run-all`. |

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
  `FORWARDS`). Where Keycloak and the API have names the functions can
  reach directly, build with those names and an empty `FORWARDS`.
- The start script creates `/mnt/data/<service>`, hands it and `/mnt/data`
  (FYLO writes its owner lease next to the root) to the user
  `mesail` (uid 10001) and drops root before `ty start` when it runs as root
  (Floci).

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
  dev host, Docker volumes live on the VM's data disk,
  `/Volumes/HANGAR/mesail/data/mesail-data.raw` since 2026-11-21; it was on
  ATLAS before, and ATLAS now holds production data only).
- No published ports: the router reaches it on the network `backend`.
- Compose settings: `MESAIL_REALTIME_GATEWAY_SECRET` (generated for each
  run), `MESAIL_REALTIME_ORIGINS` = `MESAIL_PUBLIC_URL`, and
  `YON_SERVICE_CLIENT_SECRET` = `MESAIL_SERVICE_CLIENT_SECRET`.

### Functions (`deploy.sh`)

- One function `mesail-<service>` per Yon app (not for a `"kind":
  "container"` entry such as `realtime`), `PackageType=Image`, arm64,
  memory and timeout from `stack.json`.
- FYLO owners get one EFS access point each on the file system named by
  `stack.json` `fileSystem.creationToken` (`mesail-data-2`), mounted at
  `/mnt/data`, a VPC configuration (Floci needs one; the IDs are
  placeholders), and **reserved concurrency 1**, so only one warm instance
  writes the root (ADR 0004 section 4).
- FYLO owners have a 60 second timeout (`stack.json`), under the router's
  70 second upstream timeout: Floci stops and replaces a container at the
  function's timeout.
- **One live container per FYLO root** (ADR 0004 section 4, SLICES K8):
  `fylo exec --loop` holds its root's lease until the process exits, so a
  second container of the same function would answer `EROOTLOCKED` ("store
  is not available") on every call. `scripts/floci/writers.sh` keeps one:
  - before `deploy.sh` updates an existing FYLO function, it stops that
    function's running containers (`writers.sh before-redeploy <service>`);
  - while `stack.sh` runs the tests, `writers.sh watch 5` checks every 5
    seconds for a second container of a FYLO function, stops the older one
    and logs it to `writers.log` in the run's log folder;
  - `writers.sh check` does one check by hand (`eval "$(mesail-devhost env)"`
    first, for the dev host's Docker).
  The guard only stops containers. It never removes a container, a volume or
  any FYLO data; Floci drops a stopped container from its pool by itself.
- Each access point's client token is `<creationToken>-<service>`: Floci
  keeps client tokens unique across file systems.
- `mail-out` gets a new test DKIM key (Ed25519) per hosted domain at each
  deploy, as `MESAIL_SECRET_DKIM_<DOMAIN>_<SELECTOR>` (the PEM text). Without
  a key, mail-out defers every hosted send before it delivers (the native
  run writes the same keys to a folder).
- `mail-in` and `mail-out` share one access point (`"fileSystemShare":
  "mail"` in `stack.json`). Each keeps its own FYLO root
  (`/mnt/data/mail-in`, `/mnt/data/mail-out`), and both use the blob folder
  `/mnt/data/blobs`: mail-out writes the signed message there and mail-in
  copies it on the hand-off (`stack.json` `lambdaSettings`, Floci only).
  Without it every email between hosted domains stops at mail-out or mail-in.
- `attachments` uses the same access point (slice OP7), with its own FYLO
  root `/mnt/data/attachments` and the same blob folder:
  `YON_ATTACHMENTS_ADAPTER=local`, `YON_ATTACHMENTS_FOLDER=/mnt/data/blobs`,
  `YON_ATTACHMENTS_LINK_BASE=http://127.0.0.1:18080/blobs` (`stack.json`
  `lambdaSettings`) and the shared `MESAIL_LINK_SECRET`. No bucket and no
  S3 setting (ADR 0007 section 7). The link base is the router on the Mac,
  so the link works for a client there; it assumes the default router port
  18080.
- **The router's `/blobs` on Floci** (`scripts/floci/blobs.sh`, run by
  `stack.sh` after `deploy.sh`; the steps are in OPERATIONS.md section 3):
  it finds the `mail` access point's id with `describe-access-points`, then
  the one Docker volume named `...efs-<that id>-...`. Floci makes that
  volume only when a function that uses it first starts, so when none
  exists yet the script wakes `attachments` once and prints a warning; none
  or several volumes after that stop the run. It checks that Docker Engine
  is 26 or newer, then runs `deploy/floci/compose.blobs.yaml` for two
  services only: `blobs-folder` (one-shot, makes `blobs` in the volume,
  owned by uid 10001) and the router, re-created with only that folder
  mounted at `/blobs` (compose `subpath: blobs`, read-write for uploads),
  `MESAIL_ROUTER_BLOBS_FOLDER=/blobs`, `_BASE`
  `http://127.0.0.1:<router port>/blobs` and `_SECRET` `MESAIL_LINK_SECRET`.
  The router never sees the FYLO roots in the same volume. When
  `attachments` is not deployed, `blobs.sh` does nothing and `/blobs`
  answers 404. The dev host's supervisor repairs only its own project
  `mesail-floci`, so it never re-creates this router. `down.sh` keeps the
  volume; in Colima it removes every `floci-aws-efs-*` volume after the
  router is gone.
- In Floci the access point's root directory and user are not applied: each
  access point is its own Docker volume `floci-aws-efs-<id>-<hash>`.
- **A fresh data set without removing anything.** The FYLO data in the dev
  host stays between runs. The file system `mesail-data` holds members with
  the random Keycloak subjects of the old per-domain realms, which break
  tests. Since the switch to one realm (L5) the stack uses a new file system,
  `mesail-data-2`, so every service starts with empty data once; the old
  file system and its volumes stay untouched. The dev people have fixed ids
  now, so the data stays valid from run to run. If you ever need another
  fresh set, change `creationToken` again (`mesail-data-3`, ...). Never
  remove the old volumes in the dev host. The realtime volume
  `mesail-realtime-data` is kept as it is.

### Router (`deploy/router`)

One HTTP origin, like FitNest's gateway:

| Path | Goes to |
| --- | --- |
| `/_router/ready` | The router itself. |
| `/api/v1/realtime/gateway/...` (and any `MESAIL_ROUTER_BLOCKED` prefix) | Always 404: the realtime gateway's private routes are never public. |
| `MESAIL_ROUTER_PASSTHROUGH` prefixes | Proxied as they are, with streaming and WebSocket upgrades. When several prefixes match, the longest one wins. They are checked before `/api/v1/<service>/`, so a service reached this way needs no entry in `MESAIL_ROUTER_SERVICES`. In `docker-compose.yml`: `/api/v1/realtime/socket` and `/api/v1/realtime/stream` go to the gateway (`http://realtime:8116`), and every other `/api/v1/realtime/` path goes to the realtime Yon app (`http://realtime:8106`). The gateway's management port 8117 is never routed. |
| `/blobs/<domain>/<name>?<signature>` | `kit/blobs` `LocalHandler` (GET and PUT only) when `MESAIL_ROUTER_BLOBS_FOLDER`, `_BASE` and `_SECRET` (32+ characters) are set: the signed upload and download links of the local blob store. The values match the attachments app's `YON_ATTACHMENTS_FOLDER`, `_LINK_BASE` and `_LINK_SECRET`. Without them, 404. |
| `/api/v1/<service>/...` | Lambda `mesail-<service>`, invoked with an HTTP API v2.0 event. The event's `host` header is `localhost` (Tachyon answers only local names) and the original host is in `x-forwarded-host`. Unknown service: 404. Body over 6 MiB: 413. A function error: 502. |
| `/idp/...` | Keycloak (served under `/idp`), with the original `Host`, only when `MESAIL_ROUTER_KEYCLOAK_URL` is set. Both stacks leave it unset: only the services talk to Keycloak, so `/idp/` answers 404. |
| `GET /_dev/dns-query` | Only when `MESAIL_ROUTER_DEV_DNS_DOMAINS` is set (native runs): a DNS-over-HTTPS (JSON) responder for those test domains, so the directory's domain check can pass locally: MX `10 <MESAIL_ROUTER_DEV_DNS_MAIL_HOST>.`, TXT `v=spf1 <MESAIL_ROUTER_DEV_DNS_SPF> -all`, any `<selector>._domainkey` a placeholder DKIM key, `_dmarc` `v=DMARC1; p=reject`; anything else NXDOMAIN. Otherwise 404. |
| `/admin`, `/admin/...` | The `web-admin` bundle in the bucket `mesail-static` under `admin/`. |
| everything else | The `web` bundle under `web/`. A missing page (no file extension, or a browser asking for HTML) gets the app's `index.html`. |

Limits at the router:

- **Per-address rate limit** (`MESAIL_ROUTER_RATE_LIMIT`, default 10 per
  minute per client address; both test stacks set 1000, because the
  blackbox tests sign in more than 100 times) on `POST /api/v1/identity/session` and
  `POST /api/v1/identity/session/refresh`
  (`MESAIL_ROUTER_LIMITED_ROUTES`). Over the limit: 429 with `Retry-After`.
  Identity keeps no attempt counter; Keycloak's brute-force detection
  covers each account.
- **One writer per FYLO root:** in Lambda mode the router sends one request
  at a time to each FYLO-owning function (`MESAIL_ROUTER_SINGLE_WRITERS`,
  filled from `stack.json`) and queues the others, so reserved concurrency 1
  never answers 429 to a user. A request keeps its function's place until
  the function answers, even when the caller stops waiting, because the
  function keeps running. Other functions take up to 1000 at once.
- **User requests before background calls** (slice K3): each function has
  two queues. Background calls are the scheduled routes
  `/api/v1/<service>/outbox/relay`, `/outbox/dead/requeue` and
  `/jobs/<job>`; everything else is a user request. When the function is
  free, a waiting user request always goes next, so a sign-in's `GET
  /directory/people/{id}` never waits behind a queue of removal or relay
  calls. A background call that has waited more than 60 seconds goes next
  once (then a user request gets its turn again), so background work never
  starves. A read cannot skip the queue: the function runs one call at a
  time. A caller that stops waiting leaves its queue.
- **Event pushes never wait:** `POST /api/v1/<service>/events` to a
  FYLO-owning function that is busy gets 429 `function-busy` `[MS5502]` at
  once, as a throttled Lambda does. The producer's outbox keeps the event and
  the relay schedule sends it again. Without this, drain on write deadlocks:
  a send in `messaging` pushes to `mail-out/events`, mail-out's intake
  pushes `email.queued` back to `messaging/events`, and that push would wait
  for the slot the send still holds. Floci's own throttle (429
  `TooManyRequestsException`) is also answered as 429 `function-busy`
  `[MS5503]`. Both numbers are in `docs/mesail/ERRORS.md`.
- **An event handler never calls back into its producer:** the router
  cannot break that cycle. A producer holds its own slot while it pushes,
  so a handler that calls the producer waits until the push times out
  (10 s), and every call queued behind the handler waits too. So `email.received`
  carries the email's content, and `messaging` does not call
  `mail-in` for it (EVENTS.md section 4, item 4).

Static replies carry `Content-Security-Policy`, `X-Content-Type-Options`,
`X-Frame-Options` and `Referrer-Policy`.

### Keycloak, secrets and people

Keycloak is the `mesail-keycloak` image (Keycloak 26.8.2 on SQLite, owned by
`deploy/keycloak`), as it is: one realm `mesail`, no domain realms. It runs
with the dev people on (`MESAIL_KEYCLOAK_DEV_USERS=true`), its SQLite file
on a tmpfs (no volume, so every start begins with a fresh realm), and
listens on port 8080 under `/idp` inside the network. Its hostname is
`http://keycloak.localhost:18180/idp`: the address the Lambdas and the
realtime container use (through their local forwarder to `keycloak:8080`).
So the realm's issuer, `http://keycloak.localhost:18180/idp/realms/mesail`,
is exactly what `identity` checks. Only `identity` and `directory` talk to
Keycloak. The browser and the tests never do.

**Secrets.** Each run makes one set of random secrets in a private temporary
file (mode 600, removed at the end, never in a log), or uses the file named
by `MESAIL_SECRETS_FILE`. `stack.json` `generatedSecrets` names each secret
and its kind: `hex` (32 random bytes as hex), `random-32-base64` (32 random
bytes as base64) or `p256-pkcs8-base64` (a new P-256 key, PKCS #8 DER as one
base64 line).

| Generated | Keycloak reads | Services read (only the apps that use the name) |
| --- | --- | --- |
| `MESAIL_IDENTITY_CLIENT_SECRET` | same name | `YON_IDENTITY_CLIENT_SECRET` |
| `MESAIL_ACCOUNT_CLIENT_SECRET` | same name | `YON_IDENTITY_ACCOUNT_SECRET` |
| `MESAIL_KEYCLOAK_DIRECTORY_SECRET` | same name | `YON_DIRECTORY_ADMIN_SECRET` |
| `MESAIL_SERVICE_CLIENT_SECRET` | none | `YON_SERVICE_CLIENT_SECRET`: identity checks it at `POST /service-token`; every app that calls another app sends it there. The scheduler uses it too. |
| `MESAIL_IDENTITY_SESSION_KEY` | none | `YON_IDENTITY_SESSION_KEY` (identity) |
| `MESAIL_IDENTITY_SIGNING_KEY` | none | `YON_IDENTITY_SIGNING_KEY` (identity signs every Mesail token with it) |
| `MESAIL_LINK_SECRET` | none | `YON_ATTACHMENTS_LINK_SECRET`, `YON_MAILIN_BLOBS_LINK_SECRET` (and the router's `/blobs`) |
| `MESAIL_REALTIME_GATEWAY_SECRET` | none | The realtime container: the gateway, and the Yon app as `YON_REALTIME_GATEWAY_SECRET` |

The map is `secrets` in `stack.json`. A service gets a name when its app
folder mentions it (`YON_NAME` or its dotted form, in `.env.example`, code or
`application.json`). In Floci the values are the functions' environment
variables, which the start script writes into the app's `.env` at cold
start; natively they go straight into the copied `.env` (an app's own `.env`
value wins). `YON_IDENTITY_SIGNING_PREVIOUS` and `YON_SERVICE_CLIENT_PREVIOUS`
are only for a key rotation and are never generated (see `.env.example`).

**Dev settings.** `stack.json` `appSettings` holds plain settings that only
the test stacks set: `directory` gets `YON_DIRECTORY_DEVELOPMENTSEED=true`
(Floci: the function's environment; native: the copied `.env`).

**People** (`stack.json` `people`; one sign-in per person, the realm's dev
people with fixed Keycloak ids):

| Token | Sign-in | Person id | Memberships |
| --- | --- | --- | --- |
| `MESAIL_TOKEN_ANA` | `ana` | `3e5a11d0-0000-4000-8000-000000000001` | `example.com` (admin, default), `example.org` |
| `MESAIL_TOKEN_BO` | `bo` | `...000000000002` | `example.com` |
| `MESAIL_TOKEN_EVE` | `eve` | `...000000000003` | `example.org` |
| `MESAIL_TOKEN_CAROL` | `carol` | `...000000000004` | `partner.test`, a `pending` domain, so `carol@partner.test` stays external |
| `MESAIL_TOKEN_OPERATOR` | `operator` | `...000000000005` | none; Keycloak role `mesail-operator` |

Passwords are `dev-<name>-password` (dev only, in `stack.json`).

**Seeding the directory** (`mesail_seed_directory` in
`scripts/floci/common.sh`, before the tokens; every call goes through the
router, and every step can run again):

1. A service token from `identity`: `POST /api/v1/identity/service-token`
   with `{"clientId": "mesail-service", "clientSecret":
   MESAIL_SERVICE_CLIENT_SECRET, "domainId": "*"}`.
2. With it, `POST /api/v1/directory/people/seed` for `operator` (no
   memberships). `identity` signs in only people that the directory knows.
3. `operator` signs in (`POST /api/v1/identity/session`) and adds the
   domains: `hostedDomains` (`example.com`, `example.org`) are added and
   verified; `pendingDomains` (`partner.test`) are added and never
   verified, so they stay `pending`.
4. With the service token, the seed call for `ana`, `bo`, `eve` and `carol`.
   `carol` is a member of `partner.test`. Only `active` domains count as
   hosted for recipients and `mail-out`'s hand-off, so `carol@partner.test`
   resolves `external`, and carol can still sign in to set her domain up.

**Tokens** (`tokens.sh`): each person signs in through `identity`
(`POST /api/v1/identity/session`, login = the username), and the script
prints `MESAIL_TOKEN_<NAME>`, `MESAIL_PASSWORD_<NAME>` and
`MESAIL_SERVICE_CLIENT_SECRET` for the blackbox tests. It never calls
Keycloak. Before the seed, `test.sh` calls every function's health route
once, so no test waits for a cold start (about 10 s in Floci, longer than the
tests' 10 s client timeout). Tokens last 60 seconds, so `test.sh` makes fresh ones before each
test file and runs the files one by one (with `MESAIL_TEST_RUN`, one run
with one set of tokens).

**Seed retries** (slice OP8): a seed call that does not answer `200` is
tried again up to 3 times, 5 s apart, with a fresh service token. Every try
is logged with its status, time, code, error number and clock time, for
example `seed person operator: 503 time=5.1s code=unavailable number=MS1048
at=10:12:04Z, retry 1 of 3 in 5 s`, and the last line says `(after retry N
of 3)`. When every try fails the warning stays. When the answer is a
store-unavailable number of the app (`stack.json` `storeUnavailable`, for
example `MS1048`), the state is captured BEFORE the first retry (next
section), once per app per run.

### FYLO trace and capture before retry (slice OP8)

Used to find why the first store call on a fresh `directory` takes over
5 s (MS1048). Plan: `fylo-capture-before-retry.md` in the MESAIL Build
Coordinator's notes.

With `MESAIL_FYLO_TRACE=1`, `build.sh` passes the build argument
`MESAIL_FYLO_TRACE=1` to the Lambda and realtime images (default `0`: the
image is unchanged). Then:

- `/usr/local/bin/fylo` is a bash wrapper (`deploy/lambda/fylo-trace.sh`);
  the real binary is `/usr/local/lib/mesail/fylo.real`. It writes one line
  `<UTC time> fylo-start pid=<pid> subcommand=<word>` (never the arguments
  or the environment) to `/tmp/fylo-trace-<pid>.log` and to
  `/tmp/fylo-trace-starts.log`, sends only fylo's stderr, with a UTC time
  on each line, to `/tmp/fylo-trace-<pid>.log`, and ends with `exec`, so
  fylo keeps the wrapper's pid and TACHYON's SIGKILL reaches fylo itself.
  stdin and stdout are not touched (they carry `exec --loop`). A trace with
  no end and an MS1048 means TACHYON killed that fylo.
- The start scripts (`start.sh`, `realtime-start.sh`) run
  `deploy/lambda/fylo-start-snapshot.sh` BEFORE the app starts (so no fylo
  of this container runs yet): a read-only tar of the root and its owner
  files `.<root>.fylo-root-owner.lock*` to `/tmp/fylo-root-at-start.tar`,
  and one line (UTC time, bytes, seconds) to `/tmp/fylo-trace-start.log`.
  A root over 200 MB is skipped, and the line says so. With
  `MESAIL_FYLO_TRACE=1`, the start snapshot delays a cold start: 15 s was
  seen for directory's 12 MB root under Mac memory pressure (2026-11-21).
- `build.sh` runs `deploy/lambda/fylo-trace_test.sh` in the `directory`
  image (`trace-wrapper-check.log`): same pid, hidden arguments, stderr
  only to the log, and no fylo left after a SIGKILL.
- `down.sh` copies each container's start line and fylo start list to
  `<log dir>/fylo-trace/<container>/` and prints whether the snapshot came
  before the first fylo start.

**Capture before retry** (`scripts/floci/capture.sh <service> <number>`,
called by the seed retry, or by hand): it saves the state before any retry
or other call to that app:

1. `docker compose stop` (never `rm`) `scheduler` and `scheduler-daily`,
   and at once `docker stop` (never `rm`) the app's containers
   `floci-aws-mesail-<service>-<id>`.
2. Into `<log dir>/capture-<HHMMSS>/`: `containers.txt`; `fylo-trace/<container>/`
   with the start snapshot, the start line, the fylo start list, every
   `fylo-trace-<pid>.log` and the container log (`docker cp` from the
   stopped container); `volume.txt` (the app's EFS volume, found as for the
   blob folder); `root-after.tar`, the root and its owner files read through
   a one-shot container that mounts the volume READ-ONLY; and
   `owner-files/`.
3. `timing.txt`: a one-shot container from the app's own image, as uid
   10001, with only the app's `FYLO_` variables (values never printed),
   unpacks the start snapshot (or `root-after.tar` when there is none; the
   first line says which) as `/mnt/data/<service>` with its owner files
   beside it (the app's own names and layout), in its own Docker volume
   `mesail-op8-capture` (`stack.json` `captureVolume`, made with `docker
   volume create` on the dev host's ext4 data disk; never a new root in the
   app's volume, which is only mounted read-only), starts the real fylo with
   TACHYON 26.46.02's options (`exec --loop --root ... --max-request-bytes
   1048576 --max-response-bytes 8388608`) with no 5 s limit (10 minutes per
   answer), and times start plus handshake apart from the failed request's
   store steps in order (`stack.json` `captureSteps`; for `directory`: read
   `local-parts`, write `local-parts`, find in `people`, write `people`),
   then fylo's stderr. Two guards keep the copy away from the real data,
   whose names are the same: no container that writes or deletes at
   `/mnt/data` mounts the app's volume (the read-only tar mounts it at
   `/source`), and the unpack and the delete steps first check that
   `/mnt/data/.mesail-op8-capture-marker` holds `mesail-op8-capture`
   (written when the volume is new and empty, then never deleted), and
   refuse otherwise. It deletes only the paths the copy made (each
   logged as `deleted <path>`), refuses to start when one of them is
   already there, and prints the volume's size; the volume stays (no
   script removes dev host volumes, so it is not an anonymous `--rm`
   volume).
4. `docker compose start` the scheduler services again; then the retry.

The tars stay on the Mac until FYLO says they can go (dev test data only).
`scripts/floci/capture_test.sh` (run by `scripts/gates.sh`) checks the
order with a fake MS1048 and fake `docker`, `curl` and `aws`.

### SMTP edge (`Dockerfile` target `smtp`, slice OP8)

`edge/smtp` runs as the container `smtp` from
`deploy/floci/compose.blobs.yaml`, started by `blobs.sh` with the router
once the blob folder exists: a static Go binary on `scratch`, user 10001,
on `mesail_floci_backend`, port 25 published on `127.0.0.1:12525` only
(`MESAIL_SMTP_PORT`). Settings: `MESAIL_DIRECTORY_URL` and
`MESAIL_MAIL_IN_URL` are the router (`http://router:8080`),
`MESAIL_RAW_STORE_DIR` is the shared blob folder (the `blobs` subpath of
the `mail` volume at `/blobs`, the folder `mail-in` reads as
`/mnt/data/blobs`), and the service token comes from `identity` with the
`mesail-service` secret as a compose secret file. `blobs.sh` waits up to
60 s for the `220` banner and writes the address to `<log dir>/smtp-address`;
`test.sh` passes it as `MESAIL_SMTP_ADDR`, so `tests/blackbox/Smtp_test.go`
runs. If the banner does not come (in the dev host: no Lima forward for
12525), the run warns and the SMTP tests skip. `down.sh` stops it in the
dev host and removes it with `MESAIL_VM=colima`. Production SMTP
(kestrel, ADR 0007 section 4) is slice OP2.

### Scheduler (Floci)

In Lambda form no `@Schedule` worker runs, so the `scheduler` container
(`Dockerfile` target `scheduler`, `deploy/lambda/scheduler.sh` on a pinned
curl image) calls every route in `stack.json` `schedules` through the router
every `scheduleSeconds` (60). Its token is an install-wide service token from
`identity` (`http://router:8080/api/v1/identity/service-token`, domain `*`,
secret `MESAIL_SERVICE_CLIENT_SECRET`). In each tick it calls every route
once, then calls again at once each route that answered `200` with `more:
true`, one call at a time per route and the routes in turn (EVENTS.md
section 4 item 5, slice K6). It stops calling a route on `more: false`, on
any other answer or on a timeout, and stops repeating at the longer of
the tick length minus 10 seconds and half the tick (50 s for 60 s ticks,
5 s for 10 s ticks). It logs one line per call (the
path, the status code and the first 200 characters of the answer, such as
`{"dropped":2,"more":false}`) and, at the end of each tick, the number of calls
per route (`calls this tick: <path>=<n> ...`). The next tick starts
`scheduleSeconds` after the last one started. Under `ty start` the
scheduler's 10 s ticks repeat for 5 s. When it gets no token (the stack is
still starting), it tries again after the tick, or after 60 s when the tick
is longer.

A schedule with `"daily": true` goes to the second container,
`scheduler-daily` (same image, ticks of 86,400 s): today only messaging's
change-log cleanup, `POST /api/v1/messaging/jobs/changes-cleanup` (slice O1).
Its first call comes as soon as `identity` gives a token, then once a day.
The route drops at most 12 old month collections per call and answers
`CleanupReport{dropped, more}`; the scheduler calls again while `more` is
`true` (for up to 86,390 s). It needs an install-wide token: a domain token
gets `403 [MS1566]`. It has `"native": false`, because under `ty start` the
messaging worker `changes-cleanup` runs it daily itself (first run 613 s
after start). The router counts it as a background call (`/jobs/<job>`).

How long a user waits behind a sweep (slice K16): the scheduler calls one
route at a time, so two apps never sweep at the same moment, and the router
lets a waiting user request go before any background call (above). So at a
tick a user request waits at most for the one background call that is
running. Every scheduled outbox sweep stops starting new documents after
2 seconds (1 second in `directory`) and answers `more: true`, so that wait is
about 2 seconds. A one-minute rule has no seconds offset, so the scheduler
needs no start times of its own: the `@Schedule` start times in
OPERATIONS "Scheduled work" are for `ty start` only.

## Ports on the Mac (127.0.0.1 only)

| Port | Service |
| --- | --- |
| 18080 | Router (`MESAIL_BASE_URL` of the blackbox tests) |
| 18180 | Keycloak under `/idp` (for checks only; the tests never call it) |
| 14566 / 4566 | Floci (AWS API) in the dev host / in Colima |
| 12525 | SMTP edge (`MESAIL_SMTP_ADDR` of the blackbox tests; the dev host needs a Lima forward for it) |

## Not done yet

- Native app targets of the Tac apps (macOS, Windows, Linux, iOS, Android).
- EventBridge schedules that call each service's `/outbox/relay` and
  `/jobs/<job>` routes in Lambda form. Until then the `scheduler` and
  `scheduler-daily` containers stand in; a rule made from `stack.json`
  `schedules` should use `rate(1 minute)` for the others and `rate(1 day)`
  for `"daily": true`, and repeat a call while it answers `more: true`.
  Such rules all fire in the same second; the one-call-at-a-time queue per
  FYLO function and the 2-second sweep budget (slice K16) keep that short.
- Event relay schedules (EventBridge) and the realtime WebSocket API.
- Deployment to kestrel through PaaS.
