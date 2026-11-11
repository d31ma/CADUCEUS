# Mesail on Floci (local AWS)

Every Yon app runs as an AWS Lambda container image. On this Mac the images run
in [Floci](https://hub.docker.com/r/floci/floci) 2.4.0, a local AWS emulator,
next to Keycloak and the Mesail router. The same images later run in Floci on
kestrel (through PaaS, like FitNest) and in AWS Lambda.

## Quick start

```sh
scripts/floci/run-all.sh                       # build, deploy, seed, run every blackbox test, tear down
MESAIL_TEST_RUN=TestEveryService scripts/floci/run-all.sh
scripts/floci/run-all.sh identity directory    # build and deploy only these functions
```

`run-all.sh` takes the locks itself:

1. It bundles the Tac apps (`web`, `web-admin`) with the pinned `ty` as a
   `tachyon-build` heavy job. A bundle that fails is a warning: the app's
   pages are not served, and the API tests still run.
2. It runs `stack.sh` as one Docker job in the `mesail-docker` root with
   `--vm=colima`. `stack.sh` starts Colima if it is stopped, builds the
   images, starts the stack, deploys the functions, seeds the realms, runs
   `go test -tags blackbox` in `tests/blackbox`, removes the stack, and stops
   Colima again if it started it.

Logs: `~/Library/Logs/local/CADUCEUS/<MESAIL_OWNER>/floci-<time>/`, one file
per step. Set `MESAIL_OWNER` to your slice name (for example `slice-m3`).

Settings (environment, or a `.env` file copied from `.env.example`):

| Name | Meaning |
| --- | --- |
| `MESAIL_OWNER` | Lock owner and log folder (default `floci-run-all`). |
| `MESAIL_TEST_RUN` | `go test -run` filter for `tests/blackbox`. |
| `MESAIL_SKIP_STATIC=1` | Skip the Tac bundles and the static upload. |
| `MESAIL_SKIP_BUILD=1` | Reuse the images that are already built. |
| `MESAIL_VM` | `colima` (the only engine so far; the `mesail` VM comes next). |

## The pieces

| Path | What it is |
| --- | --- |
| `Dockerfile` target `lambda` | One image per Yon app: `--build-arg APP=<service> --build-arg FYLO=<true/false>`. |
| `Dockerfile` target `router` | The public origin (`deploy/router`). |
| `deploy/lambda/` | Image helpers: tool install with checksums, the Lambda profile, the start script. |
| `deploy/floci/stack.json` | The Floci description: functions, memory, timeouts, FYLO owners, file system, bucket, realms and test users. |
| `deploy/floci/realm.jq` | Turns one realm of `stack.json` into a Keycloak realm. |
| `docker-compose.yml` | Floci, Keycloak and the router on the network `mesail_floci_backend`. |
| `scripts/floci/*.sh` | `build`, `up`, `deploy`, `seed-realms`, `tokens`, `test`, `down`, `stack`, `run-all`. |

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
  build writes into the image only: loopback host and, for FYLO owners,
  `"datasource": {"driver": "fylo", "url": "fylo:///mnt/data/<service>"}`.
- `YON_EGRESS_ENFORCEMENT=proxy`: Lambda cannot confine processes (TACHYON
  ADR 0113).
- The start script creates `/mnt/data/<service>`, hands it to the user
  `mesail` (uid 10001) and drops root before `ty start` when it runs as root
  (Floci). In AWS, Lambda already runs as a non-root user and the EFS access
  point (`PosixUser` 10001, root directory `/<service>`) owns the folder.

### Functions (`deploy.sh`)

- One function `mesail-<service>` per Yon app, `PackageType=Image`, arm64,
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
| `/api/v1/<service>/...` | Lambda `mesail-<service>`, invoked with an HTTP API v2.0 event. The event's `host` header is `localhost` (Tachyon answers only local names) and the original host is in `x-forwarded-host`. Unknown service: 404. Body over 6 MiB: 413. A function error: 502. |
| `/realms/...`, `/resources/...` | Keycloak, with the original `Host`. |
| `/admin`, `/admin/...` | The `web-admin` bundle in the bucket `mesail-static` under `admin/`. |
| everything else | The `web` bundle under `web/`. A missing page (no file extension, or a browser asking for HTML) gets the app's `index.html`. |

Static replies carry `Content-Security-Policy`, `X-Content-Type-Options`,
`X-Frame-Options` and `Referrer-Policy`.

In AWS the same routes become an API Gateway HTTP API (Lambda proxy
integrations, payload 2.0) plus S3 and CloudFront for the static files.

### Keycloak and test users

Upstream Keycloak 26.8.2 in `start-dev` until the Keycloak stages of the
`Dockerfile` exist (`MESAIL_KEYCLOAK_IMAGE` switches it). Its hostname is the
router's public URL, so every token's issuer is
`http://127.0.0.1:18080/realms/<domain>`; Lambdas reach it inside the Docker
network at `http://keycloak:8080` (back-channel).

`seed-realms.sh` creates one realm per entry of `stack.json`:

| Realm | Users (password `MESAIL_SEED_PASSWORD`) |
| --- | --- |
| `example.com` | `ana` (`mesail-member`, `mesail-admin`), `bo` (`mesail-member`) |
| `example.org` | `eve` (`mesail-member`) |
| `partner.test` | `carol` (`mesail-member`) |
| `mesail-operators` | `operator` (`mesail-operator`) |

Each realm has the roles `mesail-member`, `mesail-admin`, `mesail-service`
and `mesail-operator`, access tokens of 60 seconds, refresh token rotation,
and three clients: `mesail-web` (public, PKCE S256), `mesail-blackbox`
(public, password grant, 30-minute tokens, tests only) and `mesail-service`
(client credentials, role `mesail-service`). Every client adds the audience
`mesail`. `tokens.sh` prints `MESAIL_TOKEN_ANA`, `_BO`, `_EVE`, `_CAROL` and
`_OPERATOR` for the blackbox tests.

## Ports on the Mac (127.0.0.1 only)

| Port | Service |
| --- | --- |
| 18080 | Router (`MESAIL_BASE_URL` of the blackbox tests) |
| 18180 | Keycloak (admin API for seeding) |
| 4566 | Floci (AWS API) |

## Not done yet

- The `mesail` Lima VM as the engine (`MESAIL_VM=mesail`).
- The SMTP edge container (`edge/smtp`, slice M1) and `MESAIL_SMTP_ADDR`.
- Event relay schedules (EventBridge) and the realtime WebSocket API.
- Deployment to kestrel through PaaS.
