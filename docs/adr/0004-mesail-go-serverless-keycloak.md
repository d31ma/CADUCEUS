# ADR 0004: Mesail: Go Tac and Yon, one Lambda per service, domain tenancy, Keycloak

- Status: Accepted
- Date: 2026-11-10
- Supersedes: [ADR 0002](0002-typescript-frontend-go-backend.md) and
  [ADR 0003](0003-sesame-authentication-boundary.md)
- Amended by: [ADR 0005](0005-one-login-per-person.md) (one login per
  person: one Keycloak realm, membership as `directory` data, Mesail's own
  access token naming the active domain). Read sections 2 and 7 with it.
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
> Amended by ADR 0005: there is one Keycloak realm, `mesail`, for every
> person. The caller's domain comes from the `domain` claim of the Mesail
> access token that `identity` issues, not from a realm name. The rest of
> this section (chat stays in a domain, email crosses domains, hand-off)
> is unchanged.

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

### 4. Every Yon service is a Lambda container image, one writer per (service, domain)
> Amended by ADR 0007 (on-premise first): on-premise production runs these
> Lambda images in Floci on kestrel (SPEC Q18, decided), in a dedicated VM
> with no route out except one egress container. SES, S3 plus CloudFront,
> API Gateway and EventBridge below are optional cloud mappings, never
> required; ADR 0007 section 5 lists the on-premise replacements.


Measured by the `mesail-serverless` spike on 2026-11-10 (report
`~/Library/Logs/local/CADUCEUS/mesail/serverless-report.md`, Floci recipe
`floci-recipe.md`):
- FYLO on ext4 on the ATLAS raw disk (the dev host and kestrel/FitNest
  pattern) writes at 109 ms p50 (346 ms p95, stalls up to 10.9 s) against
  1.46 ms on a local disk; reads are the same. Bulk loads hit Lambda's
  300 s timeout, and the killed `fylo` process left a write lock behind
  (`ENATIVE_CONCURRENT_WRITE`).
- Opening a root takes 4 to 15 ms; `fylo` uses 5 to 35 MB; a domain's warm
  service costs about 60 to 100 MB.
- The single-writer lock works across containers on one kernel
  (`EROOTLOCKED`).
- The `fylo` and `ty` Linux binaries need glibc 2.39.
- A Lambda cannot hold server-sent events or WebSockets.

Decisions:
- **Packaging.** Each Yon app is an arm64 Lambda container image on an
  Ubuntu 24.04 base (glibc 2.39): the bundled app, `ty start`, the AWS
  Lambda Web Adapter, and the Go toolchain that built the handlers. The same
  image runs in Floci on this Mac, in Floci on kestrel and in AWS Lambda.
- **One writer per (service, domain).** In Lambda form, each FYLO-owning
  service is deployed once per hosted domain: function
  `<service>-<domainKey>` with reserved concurrency 1 and its own FYLO root
  `/mnt/data/<service>/<domain>/`. The image's entrypoint writes the app's
  `.env` (`YON_DATASOURCE_URL`) from the function's environment, because
  `ty` reads the root only from `.env`. The router sends a request to the
  function of its service (the path prefix) and its domain (the token's
  issuer, the envelope's `domainId` for events, the recipient domain for
  mail), and queues calls to one function, because a second concurrent call
  to a concurrency-1 function gets `429`. Native development runs one
  `ty start` per service for all domains (collections are already keyed by
  domain, DOMAIN.md), so this is a deployment choice, not a code change.
- **Why this and not one "domain cell" process for all features.** The
  spike proposed one writer process per domain that owns all of the
  domain's data. That gives the same isolation, but it puts every feature
  in one deploy unit, against the owner's rule that each service deploys on
  its own, and one slow feature (a bulk mail import) would block the
  domain's chat. Writers per (service, domain) keep the micro-service split,
  let different domains and different services write in parallel, and
  still give each domain its own roots: the set of a domain's roots is its
  **cell**, which moves, is backed up and is deleted as one unit (`fylo
  pack` per root), and chat still never leaves it.
- **Where live roots live: on ATLAS** (the owner's decision, 2026-11-10:
  "Keep it on ATLAS"). Live roots stay on ext4 on the ATLAS raw disk, the
  FitNest pattern: a Floci Lambda with EFS on the `mesail` dev host now,
  and on kestrel later. The measured cost is accepted and designed for:
  writes take about 109 ms (p50), 346 ms (p95) and stall up to 10.9 s, so
  one writer manages about 9 writes per second; reads are as fast as on a
  local disk. Every FYLO-owning service follows these rules:
  - **Bounded work per call.** One request or job call does at most 25
    FYLO writes and stops after 20 seconds of work, far under the Lambda
    timeout (set to 120 seconds). A job that has more to do answers its
    `Report` with `more: true`, and the next trigger continues; bulk loads
    (imports, migrations, reindexing) are always split this way.
  - **Write-lock recovery.** A writer killed by a timeout can leave FYLO's
    write lock behind (`ENATIVE_CONCURRENT_WRITE` on the next call). Because
    each root has exactly one function with reserved concurrency 1, a lock
    found at start-up can only be stale. The image's entrypoint (slice P1)
    clears a stale lock only when no `fylo` process of this container is
    alive and the lock's recorded owner is not running, logs that it did,
    and then starts `ty start`; anything else fails closed.
  - **Timeouts and retries.** Callers of a FYLO-owning route use a 30
    second timeout and retry only idempotent requests (every write has an
    idempotency key, DOMAIN.md invariant 6), with backoff, at most 3 times.
    The router queues calls to a busy function instead of failing them.
  - **Slow work goes through the outbox.** A route writes its own document
    once and returns; fan-out, indexing, notification and delivery happen
    later from the outbox relay and job routes (EVENTS.md). Chat stays
    responsive because `messaging` writes one document per message.
  - **Snapshots.** `fylo pack` of every root to `.vdbx` files on ATLAS
    before every deploy, so a deploy can roll back.
  On this Mac, native development uses the APFS disk image on ATLAS (about
  2 ms writes), so slice tests are not slowed down.
- **Realtime is the exception.** `realtime` holds open connections, which a
  Lambda cannot do. In the first release it runs as one small long-running
  container (`ty start`) everywhere (the owner's decision). The serverless
  form, as AWS does it with API Gateway WebSocket APIs (`$connect`,
  `$disconnect` and `$default` routes that invoke functions, and pushes
  through the `@connections` management API), is being researched with a
  proof on Floci or a small local connection gateway by `mesail-serverless`
  (`realtime-serverless.md`); R1 keeps a seam so that form can replace the
  container without changing `realtime`'s contract.
  Chat writes stay in `messaging`; `realtime` only fans out events.
- **Data of other services** is reached only through that service's HTTP
  contract or its events, never by opening its root. `identity` has no root
  and scales freely.
- Under Tachyon's OS egress confinement a handler cannot open raw SMTP
  sockets (TACHYON ADR 0113); Lambda runs with
  `YON_EGRESS_ENFORCEMENT=proxy`. `mail-out` delivery therefore goes through
  a relay adapter or the SMTP edge, never a socket opened inside a
  confined handler.
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
> Amended by ADR 0005: one realm `mesail`; the username is the person's
> prefix; `identity` answers its own ES256 access token (60 seconds) and
> never gives Keycloak's tokens to the browser; service tokens come from
> `identity`; a person signs in once and switches domains without a
> password. The bullets below about realms per domain, `mesail-operators`
> and per-realm secrets are replaced.

- Keycloak (OIDC) owns users, passwords, passkeys (WebAuthn), second
  factors, sessions and roles. Mesail stores no credentials. SESAME, its
  engine, its FYLO root and every `/auth/*` route are removed.
- **One realm per hosted domain**, named after the domain. Realm roles:
  `mesail-member`, `mesail-admin` (domain admin), `mesail-service`.
  Operators, who add and remove domains, sign in to the realm
  `mesail-operators` with the role `mesail-operator`. Adding a domain
  creates its realm from a template through the Keycloak admin API.
- **Keycloak is reached only from the backend** (the owner's rule). Go Tac
  in TACHYON 26.46.02 cannot run a browser OIDC flow (no redirects, no query
  string, JSON only to its own origin). So Mesail shows its own sign-in
  form, and `identity` signs in to the domain's realm server to server as
  the confidential client `mesail-identity` (direct access grants, TOTP
  through conditional OTP). It answers the access token (a JWT, 60 seconds)
  and a `sessionKey`: the refresh token encrypted with AES-GCM under a key
  from the secret store. `identity` keeps no state.
- Tac keeps the access token in memory and sends it as
  `Authorization: Bearer` (`@RequestHeader`); it keeps the `sessionKey` in
  `@SessionState` (per tab) and calls `identity`'s refresh route after a
  `401`. Cookies are not used because the Tac HTTP client sends requests
  with `credentials: 'omit'`. Trade-offs, accepted for the first release:
  no Keycloak login page, so no passkeys yet; the `sessionKey` is readable
  by script in its tab, so CSP stays strict and no third-party script is
  loaded; Keycloak's brute-force detection (per user) and the gateway's limit per client address
  guard the password route. A person with addresses in two domains signs
  in once per domain and switches with the domain switcher.
- Every Yon app has one `TokenIdentityProvider` that calls
  `oidc.Configured().Identify(...)` from `kit/oidc`. A route that needs
  the caller (domain and member) gets it from a `CallerService` (`@Service`)
  that calls the same gate, because Tachyon does not pass the identity to
  the route; each slice adds that service when its first route needs it. The gate checks
  signature (JWKS, cached in the warm handler), issuer, audience, expiry and
  `azp`, and maps realm roles to `Member`, `DomainAdmin`, `Operator` and
  `Service`. Until a verifier is configured the gate refuses every token
  (fail closed).
- Revocation within 60 seconds: access tokens live 60 seconds, so a disabled
  user's next refresh fails; `realtime` rechecks open connections every 60
  seconds. No revocation list is kept.
- **Deployment:** Keycloak runs as one always-on container (a JVM; not
  practical as a Lambda). The `mesail-keycloak` session proved Keycloak
  26.8.2 on SQLite with its own provider (develop 13e1923), which corrects
  the spike report's "PostgreSQL only": Keycloak's own docs list no SQLite
  support, so this setup is Mesail's, and it runs as exactly **one
  instance** (SQLite has one writer; Keycloak's `local` cache is not a
  cluster). A clustered Keycloak would need PostgreSQL. The realm templates
  and the image belong to `mesail-keycloak` (`deploy/keycloak/**`).
- SMTP submission for third-party mail clients needs a password that is not
  the Keycloak password; it is a later slice.

### 8. UI: DuVay for rigidity, Tailwind for fluidity, every screen size

The owner's rule: DuVay web components for rigidity, Tailwind for
fluidity, and the UI must work on every screen size, because the frontend
ships on web, macOS, Windows, Linux, iOS and Android. TACHYON's own website
already combines DuVay and Tailwind (`website/client/manifest.json`,
`website/scripts/build-duvay.mjs`, `build-styles.mjs`); Mesail follows it.

- **DuVay is the component layer.** Controls, menus, dialogs, sheets,
  fields, avatars, badges, banners and toasts are DuVay elements (`w-*`).
  They also bring keyboard and focus behavior (Escape, arrow keys, focus
  trap and return) that Go Tac cannot do today. Colors, type and radii come
  from DuVay's tokens (`--w-*`); Mesail maps its brand onto those tokens and
  never hard-codes colors.
- **DuVay is pinned.** Source: `https://github.com/d31ma/DUVAY`, release
  `v26.35.01-1`, commit `bc8b56aea7969ec3b7f4ca358320b71aecde4675` (the
  release has no assets; the source archive of the tag had SHA-256
  `087f8c40b58ddffd2860cdf6632cad5b62a8a3b8c16ecf6520eb94fdea450919` on
  2026-11-10, but GitHub does not promise archive bytes, so the commit is
  the pin; the same build is also hosted at
  `https://d31ma.github.io/DUVAY/versions/26.35.01/`). DuVay has no npm
  package. `scripts/vendor/duvay.sh` fetches that commit, checks it, builds
  one **Mesail subset** bundle from DuVay's `src/` (the components listed in
  `scripts/vendor/duvay-components.txt`, after `base.js`, which the others
  need) plus the matching per-component CSS, records every hash in a
  `PROVENANCE.json` next to them (TACHYON's website does the same with
  `build-duvay.mjs`), and writes them to
  `apps/<app>/client/shared/vendor/duvay/`. Each app's
  `client/manifest.json` declares them in `tac.scripts` /
  `tac.scriptSources` and `tac.styles` / `tac.styleSources` (`"kind":
  "vendor"`) with `package`, `version`, `license` and `sha256`; TACHYON
  refuses the page when the bytes differ (TY1403). The vendored files are
  never edited by hand. Measured at that commit: all 257 components are
  about 137 KB Brotli (of the 200 KB route script budget) and 53 KB of CSS;
  a Mesail-sized subset of 22 components is about 35 KB of script and 21 KB
  of CSS. A slice that needs another component adds it to the list (owned
  by F1) and rebuilds. DuVay is loaded as a module (`tac.scripts` loads
  `type="module"`), because its bundle defines a top-level `fs` that
  clashes with Go's `wasm_exec.js` (TACHYON ADR 0045).
- **DuVay rules.** DuVay renders in the light DOM (no shadow DOM), so
  Tailwind and page CSS reach it. DuVay also ships unprefixed utility
  classes whose names clash with Tailwind's and mean other things (DuVay
  `h-100` is 100 percent, Tailwind `h-100` is 25 rem): Mesail markup never
  uses DuVay's utility classes; Tailwind owns utilities, and DuVay's CSS
  sits in a lower `@layer duvay`. The theme follows the system
  (`w-theme="auto"`, with `light`, `dark` and `high-contrast` available);
  `duvay.js` would force `light` on first load, so Mesail does not load it,
  or sets the stored theme before first paint (TACHYON request B7). Native
  targets set DuVay's OS skin (`w-os` = `ios`, `android`, `macos`,
  `windows` or `linux`). For values that change often, a Tac view sets the
  element's property rather than its attribute, because a changed observed
  attribute rebuilds the element. Inline `@event` and `on<name>` handlers
  of DuVay use `new Function`, which the CSP forbids; Mesail binds events
  only with Tac's `on:` and checks the browser console for CSP errors in
  the responsive test. DuVay's RTL support is not tested upstream and it has
  no `forced-colors` support, so Mesail's responsive test covers RTL and
  Windows high-contrast itself.
- **Tailwind is the layout layer.** Grid, flex, gap, spacing, sizing and
  breakpoints are Tailwind classes in `tac.html`. Tailwind 4.3.5's
  standalone CLI (pinned version and SHA-256; no Node packages) runs without
  Preflight, and a small Go tool, `tools/styles`, writes the generated rules
  for each component into a marked region of that component's `tac.css`
  (TACHYON's website pattern), with the shared `@property` blocks in the
  foundation stylesheet. `tools/styles --check` fails when a generated
  region is stale; it is a gate. Tailwind is not used for colors or type;
  those come from DuVay tokens. The breakpoints are DuVay's, set in
  Tailwind's `@theme`: `sm` 600, `md` 960, `lg` 1280, `xl` 1920 and `2xl`
  2560 px; panes also use container queries. The foundation stylesheet
  declares `@layer duvay, theme, base, components, utilities` and imports
  DuVay's CSS into `layer(duvay)`.
- **Safe areas.** Tac pages get no `viewport-fit=cover` today, so the shell
  uses TACHYON's `--tac-titlebar-height`, `--tac-titlebar-leading-inset`
  and `--tac-titlebar-trailing-inset` (with `env(safe-area-inset-*)` as the
  web fallback) and marks the title area with `data-tac-drag-region`.
- **One DuVay bundle only.** Some DuVay modules register their element
  without checking whether it exists, so the full bundle is never loaded
  beside the subset. DuVay's icon set is registered (as the `ms` set)
  before any component renders. The page has no platform marker, so the
  shell sets `w-os`, `w-theme` and `w-density` itself.
- **Mockups and app share one component list.** `mesail-ux` builds its
  HTML mockups in `docs/mesail/ux/mockups/build/`; the app's bundle is
  built only by F1's `scripts/vendor/duvay.sh` from
  `scripts/vendor/duvay-components.txt`, which takes the mockups' list.
- **Events.** TACHYON 26.46.02 allows only `a-z`, `0-9` and `:` in event
  names (TY1306), so Mesail uses DuVay events such as `on:change`,
  `on:input`, `on:close` and `on:toggle`, and passes values, not events, to
  Go (`on:change="x.set($event.detail.value)"`). `w-text-field` is used for
  text input (`w-input` fires `input` twice).
- **Platforms.** Every target is a web view around the same bundle
  (WKWebView, WebView2, WebKitGTK, Android WebView). Component CSS compiles
  to `@scope`, so the minimum versions are Safari and iOS 17.4, WebView2 and
  Android WebView 118, and WebKitGTK 2.44.
- **Responsive and accessible, as a gate.** Every Tac route is checked at
  320, 360, 375, 414, 768, 1024, 1280, 1440, 1920 and 2560 px wide: no
  horizontal scrolling, no clipped or overlapping text, every touch target
  at least 44 by 44 px, the layout of SPEC AC-UI-1 at each width, safe-area
  insets respected (`env(safe-area-inset-*)`), `prefers-reduced-motion` and
  dark mode honored, and right-to-left layout intact. The check is a Go test
  in `tests/responsive/` that drives a headless browser against the built
  `dist/web` (no JavaScript test code). Native builds for the six targets are
  release gates, not slice gates.

### 9. Environments
> Amended by ADR 0007: on-premise (kestrel) is the primary target; AWS is
> an optional later target that no code may assume.

- **This Mac, development and slice tests:** native macOS, no VM. Each Yon
  app runs with `ty start` behind a small native dev router on one origin;
  FYLO roots are in the APFS disk image on ATLAS, attached at
  `~/Developer/AtlasFylo/mesail` (one folder per slice); Keycloak runs
  natively when a test needs a real token.
- **This Mac, integration:** every Yon service as a Lambda in Floci inside
  the `mesail` Lima VM, with EFS volumes on an ext4 data disk whose raw file
  is on ATLAS. Linux VMs are used only for this run, Lambda image builds and
  workflow dry runs, never as a per-slice gate.
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
- TACHYON: an `@HTTPClient` option for same-origin credentials, so a
  session key can live in an HttpOnly cookie instead of script-readable
  storage.
- TACHYON (found by slice F1 in the 26.46.02 Tac runtime): a `Navigation`
  capability for Go services (read the query, assign, replace the path); a
  page timer (`@Every`) that runs inside a page call; HTTP calls from
  `@OnStart`; ARIA attributes that take Go booleans; key filters on events
  (`on:keydown.escape`) and focus control (focus, focus trap, focus
  return). Details: `slice-f1-tachyon-gaps.md` in the planning notes.
- TACHYON: a Push capability for Go Tac services (PushManager subscribe
  with a VAPID key, returning endpoint, `p256dh` and `auth`, and a service
  worker push handler that shows the notification), and native push
  (APNs, FCM, WNS) on the native targets. Without it Mesail ships in-app
  notifications and the email summary only (found by slice F4).
- TACHYON: `viewport-fit=cover` in the injected viewport meta (or a manifest
  option), and `--tac-safe-area-*` properties from the iOS and Android
  hosts, so a bottom bar never sits under the home indicator.
- TACHYON, for DuVay and Tailwind in Go Tac apps: allow `-` in event names
  (TY1306); a `prop:<name>=` binding to set properties on custom elements;
  a `ty styles [--check]` command configured by a `tac.tailwind` manifest
  member (today a website-only script); a `tac.budgets.style` budget; vendor
  scripts in `tac.blockingScripts` so a saved theme applies before the
  first paint; DuVay tag roles for the accessibility check (TY1336); the
  responsive-widths check as a `ty` command for apps; and a test that
  stateful custom elements keep their state when a watched attribute
  redraws the page.
- TACHYON (bugs and gaps found by slices M3 and R1, 2026-11-11): the Go
  egress guard pipes a kept-alive plain-HTTP connection to the first
  target it saw, so a reused connection reaches the wrong service (Mesail
  turns keep-alives off with `kit/transport` until it is fixed); `ty test`
  does not apply the required-field rule, the package-main rule for
  `@HandlesError` and `@FromBody` types, or TY1314, which `ty start` and
  `ty bundle` do; the built-in `401` answers plain text, so apps cannot
  give one error format for every status.
- DUVAY: emit kebab-case event names only, and publish release assets
  (the built bundles and their SHA-256) with each release.
- TACHYON: mount separately built Tac apps under route prefixes, or split
  Go Wasm per route, so a frontend feature can be deployed alone.
- TACHYON: more than one FYLO datasource per app, so a service can keep one
  root per domain.
- TACHYON: a topic bus shared by several servers or Lambda instances.
- FYLO: publish the Go client as a versioned module with an option for
  `--exclusive-root`; a change feed for outbox relays; qualify EFS.
