# ADR 0004: Mesail: Go Tac and Yon, one Lambda per service, domain tenancy, Keycloak

- Status: Accepted
- Date: 2026-11-10
- Supersedes: [ADR 0002](0002-typescript-frontend-go-backend.md) and
  [ADR 0003](0003-sesame-authentication-boundary.md)
- Spec: [docs/mesail/SPEC.md](../mesail/SPEC.md). UX: `docs/mesail/ux/`.

## Context

CADUCEUS becomes **Mesail**: one app that is both a team messenger and an
email client and server for several hosted mail domains. The owner set
these requirements:

1. Use TACHYON 26.46.02 and FYLO 26.45.07.
2. Write the frontend (Tac) and the backend (Yon) in Go, using only
   Tachyon's Go markers.
3. Serverless-first. Each backend service and each frontend feature can be
   deployed on its own.
4. Keycloak (OpenID Connect) replaces SESAME, in the same Docker image, with
   SQLite as its database if possible.
5. Each mail domain is tied 1:1 to its own messaging domain. Email is sent
   and received between domains; chat messages are not.
6. Develop and test on this Mac, with the services as Lambdas in Floci (a
   local AWS emulator) inside the `mesail` Lima VM; deploy later on kestrel
   through PaaS, the same way the owner's FitNest app runs (a Floci Lambda
   with an EFS mount whose disk is ext4 on ATLAS). Production AWS has the
   same shape: Lambda container images plus EFS.

ADR 0002 chose a TypeScript client and one long-running Go monolith with one
FYLO process. ADR 0003 made SESAME the authentication engine. Both conflict
with these requirements.

Facts that constrain the design (from reading TACHYON 26.46.02 and FYLO
26.45.07):

- **Tachyon Go markers** are line comments, `// @Name` or `// @Name(args)`,
  with an ASCII `@`, directly above the declaration (TACHYON ADR 0050). Role
  files have no `package` line; Tachyon adds `package main`. Normal Go
  packages go in one module per app, `server/packages/go.mod`, with
  third-party modules vendored (TACHYON ADR 0095). That `go.mod` may hold
  only `module`, `go`, `toolchain` and `require` lines.
- **No deploy command.** `ty bundle` builds `dist/web` and the Yon handlers;
  `ty start` serves them. Tachyon's own website API runs `ty start` on AWS
  Lambda as an arm64 container image with the AWS Lambda Web Adapter; the
  image also holds the compiler that built the handlers. Handlers are warm
  processes by default (`YON_ISOLATION=warm`, TACHYON ADR 0111).
- **Auth.** Every route declares `@Public`, `@SignedIn` or `@Allow(Role)`.
  One `@IdentityProvider` per app turns a request into
  `Identity{user, roles}`. Tachyon has no built-in JWT or OIDC check, and it
  does not pass the identity to the route.
- **Realtime.** Server-sent events (`@StreamRoute`) and WebSocket
  (`@SocketRoute`) exist, but sockets need a long-running server (32
  connections, 1 hour). Topics (`@Topic`) are a local file per server.
- **Frontend size.** All Go services of one Tac app build into one
  `services.wasm` (TinyGo for release builds). Route budgets: script 200 KB,
  WebAssembly 3584 KB (Brotli). `encoding/json` alone adds about 2.2 MB, so
  Tac code avoids it.
- **FYLO** is an embedded store. One root has exactly one writer process, on
  a disk with kernel locks (ext4, APFS); it refuses SMB and NFS shares in
  practice. EFS is not yet qualified by FYLO (the `mesail-serverless` spike
  tests it). FYLO has no change feed, no ORDER BY, no transaction across
  collections and no arrays of objects, and a collection index stops taking
  writes at about 240 MiB. Tachyon's `datasource` driver `fylo` runs one
  long-lived `fylo exec --loop` per server.

## Decision

### 1. Go everywhere, Tachyon markers only
- All Mesail application code is Go. Tac pages and components are
  `tac.html` views with Go services, remotes, models and setups. Yon
  services are Go controllers, services, stores, workers and models.
- Behavior is declared only with Tachyon's comment markers, written
  `// @Name` with an ASCII `@`. No other annotation or code-generation
  system is used.
- No TypeScript or JavaScript is written for the app. Files that Tachyon
  itself generates or requires (`.tachyon/`, runtime glue) are allowed and
  are not edited by hand.
- Pure domain logic that needs no framework (recipient rules, MIME, SMTP
  parsing, token checks, event types) lives in plain Go packages in `kit/`
  (section 6), where `go vet` and `go test` apply directly.

### 2. The domain is the tenant and the chat boundary
- A hosted mail **domain** (for example `example.com`) is the tenant. It
  has exactly one messaging space: its members, channels, direct
  conversations, threads, presence and search. There is no organization or
  workspace entity above or beside it.
- `domainId` is the lowercase domain name. Each domain is one Keycloak
  realm with the same name, so the token issuer names the domain and the
  token `sub` names the member; no service needs a lookup to know who calls
  (`kit/oidc` `Gate`, `Caller.DomainId`).
- **Chat never crosses a domain.** Every store method takes the caller's
  `domainId` from the verified token; there is no query without it. A chat
  send to anyone outside the sender's domain is refused by `messaging`,
  whatever the client sent.
- **Email crosses domains.** A member of another domain hosted on the same
  install is an `otherDomain` recipient: email only. That email goes
  through `mail-out` and `mail-in` like any email (headers, DKIM, an email
  conversation on both sides), even when it is handed over inside the
  install. The composer explains the choice ("Email · other domain").
- **Hand-off between hosted domains.** When `mail-out` sees that a
  recipient's domain is hosted on the same install, it does not open an SMTP
  connection. It hands the finished, DKIM-signed message to `mail-in`
  through `mail-in`'s internal route (`Allow(Service)`), from a durable
  queue item that stays until `mail-in` confirms it is stored. Each side
  writes only its own data; nothing writes another domain's or service's
  root.
- Recipient kinds and their allowed deliveries are one rule in
  `kit/recipients`: `member` and `channel` (same domain: chat; a member may
  also get email), `otherDomain` and `external` (email only).

### 3. One Tachyon app per service and per frontend app

```text
apps/
  identity/     Yon   token check, /me, public client config          (no FYLO)
  directory/    Yon   domains, members, addresses, aliases, resolve   (FYLO)
  channels/     Yon   channels and channel members of a domain        (FYLO)
  messaging/    Yon   conversations, messages, threads, sends         (FYLO)
  attachments/  Yon   upload and download, blob metadata              (FYLO + blob store)
  realtime/     Yon   push of live events, presence                   (FYLO)
  notify/       Yon   notifications, Web Push, email summaries        (FYLO)
  search/       Yon   index and query                                 (FYLO)
  mail-in/      Yon   ingest of received email                        (FYLO + blob store)
  mail-out/     Yon   outbound queue, DKIM, delivery, bounces         (FYLO)
  web/          Tac   the messenger UI
  web-admin/    Tac   domain and operator administration
edge/smtp/      plain Go SMTP listener (protocol adapter only)
kit/            shared plain Go packages (source of truth)
```

- Each app has its own `server/application.json` (Yon) or
  `client/manifest.json` (Tac), its own tests and its own build, and is
  deployed on its own.
- Each backend service owns an API path prefix, `/api/v1/<service>/`. One
  public origin routes by prefix (API Gateway in Floci and AWS), so the Tac
  apps see one Yon origin (`tac.yon.origin`).
- Each frontend app is static files under its own path prefix (`/` for
  `web`, `/admin/` for `web-admin`). Inside `web`, each feature (sidebar,
  conversation, composer, thread, search, notifications) is its own
  component folder with its own Go service, so slices are file-disjoint.
  Deploying one feature alone waits for TACHYON to mount separately built
  Tac apps (`tac.elements` is the first step); until then a feature ships
  inside `web`.

### 4. Every Yon service is a Lambda container image
- Each Yon app is packaged as an arm64 Lambda container image: the bundled
  app, `ty start`, the AWS Lambda Web Adapter, and the Go toolchain that
  built the handlers. The same image runs in Floci on this Mac, in Floci on
  kestrel, and in AWS Lambda.
- **FYLO roots on EFS.** Each FYLO-owning service has its own EFS access
  point mounted at `/mnt/data`, and its root is `/mnt/data/<service>/`. No
  root is shared between services.
- **One writer per root.** Every FYLO-owning function has reserved
  concurrency 1, so at most one warm instance (one `fylo exec --loop`)
  writes its root; FYLO's owner lock refuses a second writer. This trades
  throughput for correctness and is stated, not hidden.
- **Scale path: domain cells.** Tachyon allows one datasource per server,
  and collections are prefixed by domain (DOMAIN.md). So the same image can
  be deployed again for one domain (or a group of domains) with its own EFS
  access point, `/mnt/data/<service>/<domain>/`, and the gateway routes
  that domain's requests (the token issuer, or the recipient domain for
  mail) to it. The set of a domain's service functions is a **domain cell**:
  one writer per (service, domain) root, chat that never leaves the cell,
  and data that moves with the domain. The first install runs one function
  per service for all domains; cells are a deployment change, not a code
  change.
- The `mesail-serverless` spike (report
  `~/Library/Logs/local/CADUCEUS/mesail/serverless-report.md`, still a
  draft on 2026-11-10) found that FYLO has no server, replication or object
  store backend, that its lock is advisory on filesystems without locks,
  and that the earlier CADUCEUS Lambda deployment already ran FYLO on EFS
  with reserved concurrency 1. Its measurements of FYLO on EFS and of
  per-domain access points will amend this section.
- Under Tachyon's OS egress confinement a handler cannot open raw SMTP
  sockets (TACHYON ADR 0113); Lambda runs with
  `YON_EGRESS_ENFORCEMENT=proxy`. `mail-out` delivery therefore goes through
  a relay adapter or the SMTP edge, never a socket opened inside a
  confined handler.
- **Data of other services** is reached only through that service's HTTP
  contract or its events, never by opening its root.
- `identity` has no FYLO root and scales freely.
- **Realtime** in Lambda form uses an API Gateway WebSocket API: connect,
  message and disconnect are ordinary Yon routes of `realtime`, and pushes
  go through the gateway's connection management API. A single-server run
  may use Tachyon sockets instead behind the same `realtime` contract.
- **SMTP.** A Lambda cannot listen on port 25. `edge/smtp` is a small
  always-on Go program (a container next to Floci on this Mac and on
  kestrel; Amazon SES receiving in AWS) that writes the raw message to the
  blob store and calls `mail-in`. Outbound delivery uses direct-to-MX where
  port 25 is open and a relay adapter (SES or another SMTP relay) where it
  is not.
- **Static frontends** are served from S3-compatible storage behind the
  gateway or a CDN (Amplify or S3 plus CloudFront in AWS). The CDN sets the
  security headers that `ty start` would otherwise add.

### 5. Services talk by HTTP contracts and pushed events
- Each service exposes typed Go models as its contract
  ([CONTRACTS.md](../mesail/CONTRACTS.md)).
- **Events** (DOMAIN.md) use an outbox: the producer stores the event in the
  same document write as the change, and a scheduled relay (`@Schedule`, or
  an EventBridge schedule in Lambda form) pushes it over HTTP to each
  subscriber's `POST /api/v1/<service>/events` route. Delivery is
  at-least-once; consumers are idempotent by `eventId`. Subscribers are
  listed in the producer's `application.json`. This needs no broker; a
  managed bus can replace the relay later behind `kit/events`.
- Service-to-service calls use a Keycloak client-credentials token with the
  role `mesail-service` (`@Allow(Service)`).

### 6. Shared Go code: `kit/` copied into each app
- `kit/` is the single source of shared Go packages. Its module path is
  `mesail.dev/packages`, the same path every app uses for its
  `server/packages/go.mod`. `scripts/kit-sync.sh` copies the packages an
  app lists in `server/packages/kit.list` into that app's `server/packages/`
  without changing any import path.
- Copies are committed (Tachyon builds offline from the app folder) and are
  never edited by hand. `scripts/kit-sync.sh --check` fails on any drift.
  Tests for shared code live in `kit/`.
- No `go.work` file is used, because Go would pick it up inside Tachyon's
  handler builds.

### 7. Keycloak replaces SESAME
- Keycloak (OIDC) owns users, passwords, passkeys (WebAuthn), second
  factors, sessions and roles. Mesail stores no credentials. SESAME, its
  engine, its FYLO root and every `/auth/*` route are removed.
- **One realm per hosted domain**, named after the domain. Realm roles:
  `mesail-member`, `mesail-admin` (domain admin), `mesail-service`.
  Operators, who add and remove domains, sign in to the realm
  `mesail-operators` with the role `mesail-operator`. Adding a domain
  creates its realm from a template through the Keycloak admin API.
- The browser signs in with the authorization code flow and PKCE as the
  public client `mesail-web` of the domain's realm. Access tokens live 60
  seconds and are kept in memory; refresh uses refresh token rotation. A
  person with addresses in two domains has two accounts and a domain
  switcher in the UI.
- Every Yon app has one `OidcIdentityProvider` that calls
  `oidc.Configured().Identify(...)` from `kit/oidc`, and a `CallerService`
  that routes use to read the same verified caller. The gate checks
  signature (JWKS, cached in the warm handler), issuer, audience, expiry and
  `azp`, and maps realm roles to `Member`, `DomainAdmin`, `Operator` and
  `Service`. Until a verifier is configured the gate refuses every token
  (fail closed).
- Revocation within 60 seconds: access tokens live 60 seconds, so a disabled
  user's next refresh fails; `realtime` rechecks open connections every 60
  seconds. No revocation list is kept.
- **Deployment:** Keycloak runs as one always-on container (a JVM; it is
  not practical as a Lambda), in the same image as Mesail where the owner
  asked for that. Keycloak has no supported SQLite database; the
  `mesail-keycloak` session owns that question and `deploy/keycloak/**`. In
  Floci on kestrel and in AWS it runs as its own container service with a
  managed database.
- SMTP submission for third-party mail clients needs a password that is not
  the Keycloak password; it is a later slice.

### 8. Environments
- **This Mac (development and test):** every Yon service runs as a Lambda in
  Floci inside the `mesail` Lima VM, with EFS volumes on an ext4 data disk
  whose raw file is on ATLAS. Tests that run natively on macOS put FYLO
  roots in the APFS disk image on ATLAS, attached at
  `~/Developer/AtlasFylo/mesail` (one folder per slice).
- **kestrel (deployment, later):** the same images through PaaS, like
  FitNest.
- **AWS (possible later):** the same images on Lambda with EFS.

## Consequences

- One language for all app code. Shared logic is written once in `kit/`.
- Each service and frontend app has its own build, tests and deploy, so
  parallel slices work on disjoint files.
- The domain boundary is enforced in one place per concern (`kit/oidc` for
  who calls, `kit/recipients` for what may be sent, store ports for data).
- More moving parts than a monolith: ten Lambda functions, event relays, an
  API gateway and an SMTP edge.
- FYLO-owning functions run with reserved concurrency 1. Their throughput is
  one instance until a domain is moved to its own root.
- Copies of `kit/` packages exist in each app; the sync check keeps them
  identical.
- Users from CADUCEUS enrol again in Keycloak. Mail data needs a migration
  tool (a later slice).
- The legacy JavaScript `server/`, the JavaScript `client/`, the Go monolith
  (`cmd/caduceus`, `internal/`) and their tooling are removed from `develop`.
  They remain in git history (commit 4ef82f72, branch `go/rewrite`) for
  slices that port a behavior.

## Requests to dependencies

Friction found while designing, to raise with the dependency owners:

- TACHYON: verify OIDC bearer tokens in the identity provider (JWKS with a
  cache) and pass the verified identity to routes.
- TACHYON: mount separately built Tac apps under route prefixes, or split
  Go Wasm per route, so a frontend feature can be deployed alone.
- TACHYON: more than one FYLO datasource per app, so a service can keep one
  root per domain.
- TACHYON: a topic bus shared by several servers or Lambda instances.
- FYLO: publish the Go client as a versioned module with an option for
  `--exclusive-root`; a change feed for outbox relays; qualify EFS.
