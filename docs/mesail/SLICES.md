# Mesail implementation slices

- Status: Ready to hand out
- Date: 2026-11-10
- Spec: [SPEC.md](SPEC.md). Architecture: [ADR 0004](../adr/0004-mesail-go-serverless-keycloak.md).
  Domain: [DOMAIN.md](DOMAIN.md). Contracts: [CONTRACTS.md](CONTRACTS.md).
  Blackbox tests: [`tests/blackbox/`](../../tests/blackbox/README.md).

Each slice is one implementation session that follows the feature and fix
workflow: a worktree from `origin/develop`, the slice's gates, rebase, and a
fast-forward push to `develop`. A slice owns the files listed for it and
edits nothing else. Owned file sets never overlap, so slices of the same
wave run at the same time.

## Rules for every slice

1. **Ownership.** Edit only the paths your slice owns. If you need a change
   in a path another slice owns, ask that slice (through the coordinator);
   do not edit it. New files go inside your owned paths.
2. **Copies of `kit/`.** `apps/<app>/server/packages/<package>/` are
   generated copies of `kit/<package>/`. Only the slice that owns
   `kit/<package>` runs `scripts/kit-sync.sh` and commits the copies (in
   every app). Everyone else lists the packages they need in their own
   `server/packages/kit.list` and asks the owner to sync. The gate
   `scripts/kit-sync.sh --check` fails on any hand edit.
   **Outside Go modules.** `kit/go.mod` is the module file of every app's
   `server/packages/` (both are `mesail.dev/packages`). A kit package that
   needs an outside module (for example `golang.org/x/net/html` for
   `kit/richtext`) adds its `require` to `kit/go.mod` and `kit/go.sum` (the
   package owner does this; architecture reviews it). `kit-sync.sh` then
   copies both files into each app, runs `go mod tidy` and `go mod vendor`
   there, so each app keeps only the modules its packages use, vendored for
   Tachyon's offline build (TY1368); `--check` rebuilds in a temporary
   copy and compares everything, `vendor/` included. App-owned packages in
   `server/packages/` (such as directory's `dnscheck`) stay untouched and
   use outside modules only through `kit/go.mod`.
3. **Contracts first.** The routes and models in CONTRACTS.md are the
   contract. A slice that must change a contract updates CONTRACTS.md in
   the same commit and tells the coordinator, because other slices code
   against it.
4. **Code against interfaces, fake at the boundary.** Until a dependency
   slice lands, test with a fake of its contract (`// @Fake` fields in Yon
   tests, a fake `oidc.TokenVerifier`, a fake HTTP client). Never wait idle.
5. **Code rules.** The Junior Software Engineer rules in `~/.claude/CLAUDE.md`
   apply (one type per file named after it, cohesive functions, no magic
   values, no comments, typed models, CRAP at most 15). In Go, the
   `if err != nil { return ..., err }` pattern counts as Rust's `?`.
   Tachyon enforces most of them for role files (`TY13xx`).
6. **Markers.** Only Tachyon's `// @Name` comment markers, with an ASCII `@`.
7. **Tachyon and FYLO versions.** `ty` 26.46.02 and `fylo` 26.45.07 only
   (`scripts/gates.sh` checks `ty`). Pinned binaries on Falcos-Mini-2:
   `~/Developer/Tools/tachyon-26.46.02/ty` and
   `~/Developer/Tools/fylo-26.45.07/fylo`.
8. **Heavy jobs** run as
   `MAC_HEAVY_COORDINATOR=tachyon-build mac-heavy --owner <slice> -- ...`.
8a. **Tac setups.** Every `@HTTPClient` needs a setup in
   `client/setups/`. A web slice owns the setups whose names start with
   the same prefixes as its remotes and services (listed under each F
   slice).
8b. **Thin role files, workflows in plain Go.** `ty bundle` and `ty start`
   enforce TY1314 strictly: every `if err != nil` in a public method of a
   role file (controller, service, store, worker) counts as a decision, so
   a Yon app with multi-step logic does not bundle (found by slices I1 and
   A1). So every Yon slice puts its workflows in its own plain-Go package
   `kit/<service>/` (for example `kit/messaging`), copied only into its own
   app by `scripts/kit-sync.sh`, tested with `go test`, and held to the
   CRAP limit by the gate. Role files stay thin: a public method makes one
   call into that package and returns; a private helper holds at most one
   decision. The package talks to FYLO, HTTP clients and the clock through
   small Go interfaces that the role files fill in, so it is testable
   without Tachyon. Design this way from the start (wave 2 slices M3, R1,
   N1 included).
9. **FYLO on ATLAS (every FYLO-owning slice).** Live roots are on ATLAS in
   Lambda form, where a write takes about 109 ms with stalls up to 11 s
   (ADR 0004 section 4). So: at most 25 FYLO writes and 20 seconds of work
   per request or job call (answer `more: true` and continue on the next
   trigger); every write idempotent; slow work through the outbox; callers
   use a 30 s timeout and at most 3 retries with backoff.
10. **Collection names.** Build every FYLO collection name with
   `kit/collections` (`CollectionName{}.ForDomain` or `.ForDomainMonth`).
   FYLO accepts only lowercase letters, digits and `-`; the domain becomes a
   16-character hex key (DOMAIN.md, "The domain key"). Add `collections` to
   your `server/packages/kit.list`.
11. **The domain is the tenant.** Every store method takes the `domainId`
   from the verified caller (`oidc.Caller.DomainId`, the Keycloak realm
   name, which is the mail domain). Chat data never crosses a domain. Email
   crosses domains freely. See SPEC section 2 and DOMAIN.md invariant 1.
12. **FYLO roots for dev and test.** On this Mac, a test FYLO root lives in
    the shared APFS disk image on ATLAS, in your own folder:
    `~/Developer/AtlasFylo/mesail/slice-<slice id>/` (for example
    `slice-m3/`). Never put a root on `/Volumes/ATLAS` or `/Volumes/HAVEN`
    directly (SMB; FYLO refuses it), and never detach the image: other
    sessions share it. If it is not attached, attach it (Bash sandbox off):
    `diskutil image attach --mountOptions nobrowse --mountPoint ~/Developer/AtlasFylo/mesail /Volumes/ATLAS/Mesail/mesail-fylo-test.sparsebundle`.
    **How `ty test` finds the root** (agreed with slice C1): no `datasource`
    in the committed `server/application.json`; a committed
    `apps/<app>/.env.example` shows the format, and a local, ignored
    `apps/<app>/.env` holds
    `YON_DATASOURCE_DRIVER=fylo` and
    `YON_DATASOURCE_URL=fylo:///Users/<you>/Developer/AtlasFylo/mesail/slice-<id>/<app>`.
    `ty` reads only the app folder's `.env` (not the process environment,
    and a value in `application.json` cannot be overridden), and stops with
    TY2160 when the folder does not exist, so `scripts/gates.sh` (C1)
    creates the folder and the `.env` when they are missing.
    Only the integration run in Lambda form (Floci, AWS later) puts a
    root on EFS: `/mnt/data/<service>/<domain>/`, one function per
    (service, domain) (ADR 0004 section 4).
13. **Scheduled work.** A `@Schedule` worker gets no injected providers in
    TACHYON 26.46.02, so it only triggers: it fetches a service token for
    the domain it works on (`kit/oidc`, slice I1; EVENTS.md section 4) and
    POSTs to its own `@Allow(Service)` route
    (`/outbox/relay` or `/jobs/<job>`), where the work runs with injected
    services. In Lambda form an EventBridge schedule calls the same route
    (P1). Details: `docs/mesail/EVENTS.md` section 4.
14. **UI stack (frontend slices F1 to F5).** Follow ADR 0004 section 8:
    DuVay `w-*` elements for every control and overlay, DuVay tokens for
    color and type, Tailwind classes for layout and breakpoints, no
    hard-coded colors, no CSS frameworks other than these two, no
    JavaScript of our own. The DuVay files in
    `apps/<app>/client/shared/vendor/duvay/` and the generated Tailwind
    regions in `tac.css` are produced by `scripts/vendor/duvay.sh` and
    `tools/styles`; never edit them by hand. A Tac slice's gate also runs
    `tools/styles --check` and the responsive test in `tests/responsive/`
    (320 to 2560 px, 44 px touch targets, safe areas, RTL, reduced motion)
    for the routes it owns.
15. **UX.** The screens, states and copy come from `docs/mesail/ux/**`
    (owned by the `mesail-ux` session). Frontend slices follow it and do not
    edit it.

## Gates

Every slice runs, in its worktree, before it merges:

```sh
scripts/gates.sh --app <each app it owns>   # versions, doctor, gofmt, kit sync, kit vet/test/CRAP, blackbox compile, ty migrate check, ty test --coverage, ty bundle (Tac apps)
```

plus, when it owns a `kit/` package, `cd kit && go test ./<package>/...`,
and its named blackbox tests against the services started natively (see
below) once `scripts/native/run-all.sh` exists (slice P1); until then, the
compile check is enough.
`ty test --coverage` fails when a public method has no test (TY2022).

**Push only after a green gate on that exact commit.** Run the gates on the
commit you will push (after the rebase), wait for the result, and push only
when that run's log shows `rc=0` for that commit's SHA. Never push while a
gate is still running or after a gate of an older commit.

**A slice is done only when its app builds and starts.** `ty test` does not
apply every build rule (TY1314 is enforced by `ty bundle` and `ty start`).
So every slice, server-only apps included, also runs `ty bundle apps/<app>`
and starts the app with `ty start` against its test FYLO root, and checks
that `GET /api/v1/<service>/health` answers `200`. Slice C1 adds this step
to `scripts/gates.sh` for every app; until it lands, run it by hand and
name it in the report.

**Everything in a slice gate runs natively on macOS. No VM.** That means
`ty test`, `ty bundle`, `ty start`, `go test`, and the blackbox tests
against services started with `ty start` on this Mac behind the native dev
router. FYLO roots go in `~/Developer/AtlasFylo/mesail/slice-<id>/`.
When a test needs a real Keycloak token, use Keycloak running natively on
macOS (the `mesail-keycloak` session documents how). Fakes at the boundary
(rule 4) are preferred for unit tests.

**Linux VMs are not a slice gate.** Colima and the `mesail` dev host with
Floci are only for work that truly needs Linux: Lambda image builds, the
end-to-end Floci run (`scripts/floci/run-all.sh`) and `act` dry runs of the
workflows. Those run in the integration and release gates, owned by P1 and
C1, not before each slice merge.

## Wave 1 (start now, all in parallel)

### I1. Identity and token check
- **Owns:** `apps/identity/**` (except the copies of other slices' kit
  packages), `kit/oidc/**`, and every copy `apps/*/server/packages/oidc/**`.
- **Builds:** a real `oidc.TokenVerifier` (JWKS fetch and cache, RS256 and
  ES256 with the standard library, issuer allowlist, audience, `azp`,
  expiry with 30 s leeway) and `oidc.Configured()` reading its settings
  (issuer URL pattern, audience, JWKS URL) from the handler environment;
  `outgoing.hosts` for the Keycloak host. The session routes in
  CONTRACTS.md "identity" (sign in with address, password and optional
  one-time code through direct access grants; refresh and sign-out with the
  encrypted `sessionKey`; brute-force limits; nothing secret in logs) and
  `GET /identity/me`, the account settings routes (change password, set up,
  confirm and remove the 2-step code; CONTRACTS.md "identity", SPEC
  AC-ID-7 and AC-ID-8), and the client-credentials service token in
  `kit/oidc` (EVENTS.md section 4). The realm templates are NOT I1's: they
  live in `deploy/keycloak/realm/` and belong to `mesail-keycloak` (one
  realm per domain plus `mesail-operators`; clients `mesail-identity` with
  direct access grants and conditional OTP, and `mesail-service` with client
  credentials; 60 s access tokens; brute-force detection on; no browser
  client; a confidential client `mesail-account` per realm whose service
  account has only `manage-users` and `view-users`, for the account
  settings routes, and, if the stock admin API cannot set an OTP credential
  with a given secret, a narrow admin endpoint in its provider for that; a
  master-realm client `mesail-directory` that slice D1 uses to create a
  realm at run time). I1 states what the templates must contain
  and checks them against its code.
- **Depends on:** `mesail-keycloak` for the realm templates.
- **Blackbox:** `Identity_test.go`.
- **Done when:** every app's `TokenIdentityProvider` accepts a valid
  Keycloak token and refuses forged, expired and other-audience tokens.

### I2. Events and outbox
- **Owns:** `kit/events/**`, new `kit/outbox/**`, and their copies in
  `apps/*/server/packages/{events,outbox}/**`. **I2 is finished; since
  2026-11-11 slice M2 owns both packages and their copies.** M2's first
  change: the `TokenSource` takes the target domain and `HTTPPusher` pushes
  each domain's events with that domain's service token (EVENTS.md
  section 4), and `SettingsReader` reads under `yon` if I2 has not fixed
  it.
- **Owner M2's second change:** `kit/outbox` computes all six outbox
  fields together, including the booleans `outboxWaiting` and
  `outboxHasDead`, and offers the sweep filter `{"outboxWaiting": true}`
  (EVENTS.md section 2.1), so every producer gets the same bounded sweep.
  Producers that already copied C2's read-everything sweep (C2, M1) switch
  to it when it lands.
- **Builds:** one Go payload type per event in `kit/events` (today only
  the names exist in `Types.go`; payload fields are in DOMAIN.md and
  CONTRACTS.md), the outbox relay library (pending events stored in the
  producer's own document, HTTP push to subscribers listed in
  `application.json`, retries with backoff, dead-letter list), the consumer
  side (verify the `Service` caller, dedupe by `eventId` through a store
  port), and a test harness that other slices use to assert published
  events. Documents the per-app role files a service adds (an
  `EventsController` with `@Allow(Service)` and a `@Schedule` relay
  worker) in `docs/mesail/EVENTS.md`.
- **Depends on:** nothing.
- **Done when:** a two-app test shows an event pushed from one app's
  outbox to another app's `/events` route exactly once per `eventId`.

### D1. Directory
- **Owns:** `apps/directory/**` (except kit copies), `kit/recipients/**`
  and its copies.
- **Builds:** domains (an `Operator` adds one: Mesail creates its
  Keycloak realm from `mesail-keycloak`'s template through the Keycloak
  admin API, signed in as the master-realm client `mesail-directory`
  (secret `YON_DIRECTORY_ADMIN_SECRET`), and sets the shared
  `mesail-identity` client secret in the new realm; then
  DNS verify; remove), members
  (upserted from the caller's token on each request and from Keycloak
  admin events), addresses and aliases, `POST /directory/recipients/resolve`
  with the kinds `member`, `channel`, `otherDomain` and `external`
  (SPEC AC-CP-1, AC-CP-2, AC-ID-2), member search inside the caller's domain. FYLO store per DOMAIN.md.
  Publishes `member.upserted`.
- **Depends on:** I1 for real tokens, I2 for publishing (fake both until
  they land).
- **Blackbox:** `Recipients_test.go`, `TestAcId3MemberCannotUseAdminRoutes`.

### C2. Channels
- **Owns:** `apps/channels/**` (except kit copies).
- **Builds:** channels of a domain (public, private, join, leave, invite,
  archive, channel email address), SPEC AC-CH, contracts in CONTRACTS.md
  "channels", events `channel.*`. A domain has exactly one messaging space;
  there is no workspace entity.
- **Depends on:** I1, I2 (fake until they land).

### A1. Attachments and blob store
- **Owns:** `apps/attachments/**` (except kit copies), new `kit/blobs/**`
  and its copies.
- **Builds:** the blob store port with a local-folder adapter (all-in-one
  image) and an S3-compatible signed-URL adapter; upload, complete,
  download routes; size limits (AC-MS-5); content type and SHA-256 checks.
- **Depends on:** I1.

### M1. Inbound mail
- **Owns:** `apps/mail-in/**` (except kit copies), new `edge/smtp/**` (its
  own Go module), new `kit/mime/**`, `kit/mailauth/**` and `kit/richtext/**`
  and their copies. `kit/richtext` turns HTML email and chat Markdown into
  the typed blocks of CONTRACTS.md "Rich text" (with a fuzz test); M3 uses
  it and asks M1 for changes; the HTML parser is vendored
  (`golang.org/x/net/html`).
- **Builds:** the SMTP edge (RCPT TO check through `directory`, STARTTLS,
  size limits, `250` only after `mail-in` stored the raw message), MIME
  parsing with fuzz tests, SPF/DKIM/DMARC checks (AC-IN-1, 2, 4), the
  `email.received` event. Reads the CADUCEUS JS receiver
  (`background/InboundSmtpServer.js` at commit 4ef82f72) for behaviors to
  keep.
- **Depends on:** D1 (address lookup contract), A1 (blob store port), I2.
- **Blackbox:** `Smtp_test.go`.

### M2. Outbound mail
- **Owns:** `apps/mail-out/**` (except kit copies), new `kit/compose/**`,
  `kit/dkim/**`, `kit/smtpclient/**` and their copies.
- **Builds:** MIME composition with correct threading headers (AC-CP-6),
  per-domain DKIM signing with keys from the secret store (AC-DM-2), the
  delivery queue with retries, bounces and per-domain suppression
  (AC-DM-3, 4), direct-to-MX and relay adapters, the internal hand-off
  to `mail-in` for recipients in another hosted domain (ADR 0004 section
  2; never SMTP to itself), the `email.delivery.changed` event. Note: a
  confined Tachyon handler cannot open raw SMTP sockets (TACHYON ADR 0113),
  so remote delivery goes through a relay adapter or the SMTP edge. Reads the CADUCEUS JS queue
  (`background/OutboundQueueWorker.ts` at 4ef82f72).
- **Depends on:** I2, A1 (attachment bytes).

### S1. Search
- **Owns:** `apps/search/**` (except kit copies), new `kit/textindex/**`
  and its copies.
- **Builds:** the index fed by `message.*` events, query with filters and
  cursor (AC-SR-1..3), membership checked at query time against
  `channels` and `messaging` contracts (AC-SR-2); never across domains.
- **Depends on:** I2; M3 contracts (fake until M3 lands).
- **Blackbox:** `Search_test.go`.

### F1. Web shell and sign-in
- **Owns:** `apps/web/client/manifest.json`, `apps/web/client/pages/**`,
  `apps/web/client/components/app/**`, `apps/web/client/shared/**`,
  `apps/web/client/services/Shell*.go`, `apps/web/client/services/Session*.go`,
  `apps/web/client/remotes/Identity*.go`, `apps/web/client/models/{Session,ClientConfig,Me}*.go`,
  `apps/web/client/tests/{Shell,Session}*.go`,
  `apps/web/client/setups/{Shell,Session,Identity}*.go`.
- **Builds:** Mesail's own sign-in form (address, password, one-time
  code) calling `identity`'s session routes; the access token in memory and
  sent with `// @RequestHeader("Authorization")`; the `sessionKey` in a
  `// @SessionState` field; refresh after a `401` and retry once; sign-out;
  the domain switcher; the account settings screens (change password, set
  up and remove the 2-step code, following `mesail-ux`'s settings mockups,
  AC-ID-7 and AC-ID-8); the three-pane layout (AC-UI-1, 2, 4) and the
  feature slots the shell already places. Other web slices get the token from F1's
  `SessionService`.
- **Also owns (shared UI tooling, used by F2 to F5):** `scripts/vendor/**`
  (`duvay.sh`: fetch DuVay commit `bc8b56a` of release `v26.35.01-1`,
  check it, build the Mesail subset bundle and its CSS from the components
  in `duvay-components.txt`, write them with `PROVENANCE.json` and print the
  manifest SHA-256 lines; other slices ask F1 to add a component), `tools/styles/**` (the Go generator
  around the pinned Tailwind 4.3.3 standalone CLI, with `--check`), and
  `tests/responsive/**` (the Go responsive test). F1 does the first
  migration of the web shell to DuVay and Tailwind.
- **Depends on:** I1.

### F5. Admin app
- **Owns:** `apps/web-admin/**` (it runs F1's `scripts/vendor/duvay.sh
  --app web-admin` for its own vendor copy).
- **Builds:** domains (with DNS record display and verify, for operators),
  members and aliases, channel email addresses (SPEC S11, AC-DM-1, AC-CH).
- **Depends on:** D1, C2 contracts.

### P1. Lambda images and the Floci stack
- **Owns:** `Dockerfile` except the Keycloak stages (owned by
  `mesail-keycloak`), `docker-compose.yml`, `.dockerignore`, `.env.example`,
  new `deploy/lambda/**`, `deploy/floci/**`, `deploy/router/**`,
  `scripts/floci/**` and `scripts/native/**` (never `deploy/keycloak/**`).
- **Builds:** one Lambda container image per Yon app (arm64, Ubuntu 24.04
  base because `fylo` needs glibc 2.39, AWS Lambda Web Adapter, `ty start`,
  the Go toolchain that built the handlers, an entrypoint that writes the
  app's `.env` (`YON_DATASOURCE_URL=fylo:///mnt/data/<service>/<domain>`)
  from the function's environment because `ty` reads the root only from
  `.env` and Floci ignores the access point root folder), one function per
  (service, domain) with reserved concurrency 1, a router that picks the
  function by path prefix and domain and queues calls to a busy function
  (a concurrent call gets `429`), `realtime` as one long-running container
  (Lambda cannot hold connections), live roots on the ATLAS-backed EFS
  volumes (the owner's decision), Lambda timeouts of 120 s, an entrypoint
  that clears a stale FYLO write lock only when no writer is alive (ADR
  0004 section 4) and logs it, `fylo pack` snapshots to ATLAS before every
  deploy, the Floci description that runs every service in
  the `mesail` Lima VM (built by the `mesail-devhost` session), the
  path-routing front (API Gateway in Floci and AWS), static hosting for the
  Tac apps, seeded realms `example.com`, `example.org` (a second hosted
  domain) and `partner.test`, and `scripts/floci/run-all.sh` for the
  end-to-end integration run in Floci. **First**, the native path that every
  slice gate uses: `scripts/native/run-all.sh` starts every Yon app that
  exists with `ty start` on this Mac (FYLO roots under
  `~/Developer/AtlasFylo/mesail/slice-<id>/`), the static Tac apps, and a
  small native dev router on one origin that routes `/api/v1/<service>/`
  by prefix (a plain Go program in `deploy/router/`), and prints the
  `MESAIL_BASE_URL` for the blackbox tests. Deliver the native path before
  the Floci path. The
  deployment to kestrel through PaaS (like FitNest) comes later.
- **Depends on:** `mesail-devhost` (the VM), `mesail-keycloak` (Keycloak
  stages), `mesail-serverless` (the FYLO-on-EFS findings).

### C1. Local CI
- **Owns:** `.github/workflows/**`, `scripts/gates.sh`, new `scripts/ci/**`.
- **Builds:** workflows that run `scripts/gates.sh` for every app on pinned
  runner images (no `*-latest`), with pinned `ty` and `fylo` downloads and
  checksum checks; a nightly blackbox run against the compose stack.
- **Depends on:** P1 for the blackbox job.

## Wave 2 (start when I1 and I2 are on `develop`)

### M3. Messaging
- **Owns:** `apps/messaging/**` (except kit copies).
- **Builds:** conversations, participants, messages, threads, edits,
  deletes, reactions, read positions (AC-MS-1..8); the send orchestration
  for the one composer (AC-CP-1..8): re-resolve recipients through
  `directory`, create chat deliveries, emit `email.send.requested` for email
  deliveries, consume `email.delivery.changed` and `email.received`
  (threading by `Message-ID`, AC-IN-3, AC-IN-6). Partitioned collections
  per DOMAIN.md.
- **Depends on:** I1, I2, D1, C2 (contracts; fakes while they are open).
- **Blackbox:** `Messaging_test.go`.

### R1. Realtime and presence
- **Owns:** `apps/realtime/**` (except kit copies), new `edge/realtime/**`
  (a plain-Go connection gateway with its own module: a Yon
  `@SocketRoute` holds at most 32 connections and nothing can push into it,
  TACHYON ADR 0060; design in `slice-r1-design.md`), and `kit/realtime`.
- **Builds:** `realtime` as one small long-running container in the first
  release (the owner's decision): the socket route with the token in the
  first message and a 60 s recheck (AC-ID-4), the single-use SSE ticket,
  fan-out of `message.*`, `reaction.changed`, `read-position.moved`,
  presence (AC-PN-1) and typing. A connection-gateway seam (connect,
  message, disconnect and push behind one Go interface) so the serverless
  form researched by `mesail-serverless` (`realtime-serverless.md`: API
  Gateway WebSocket `$connect`/`$default`/`$disconnect` and `@connections`,
  or a small local gateway on Floci) can replace the container without a
  contract change. Read that report before you start.
- **Depends on:** I1, I2, M3 event payloads.

### N1. Notifications
- **Owns:** `apps/notify/**` (except kit copies), new `kit/webpush/**`
  (VAPID ES256 and RFC 8291 message encryption) and its copy in
  `apps/notify/server/packages/webpush/**` (synced only into `notify`).
- **Builds:** AC-PN-2..4: notification records, in-app delivery through
  `realtime`, mute, the email summary (via `email.send.requested`). The Web
  Push sender (`kit/webpush`, VAPID keys from the secret store) stays
  ready, but no browser can subscribe yet (Go Tac 26.46.02 has no
  PushManager), so the subscription route is kept and unused in the first
  release.
- **Depends on:** I2, M3 events, M2 contract.

### F2. Web conversations
- **Owns:** `apps/web/client/components/{sidebar,conversation}/**`,
  `apps/web/client/services/{Sidebar,Conversation,Realtime}*.go`,
  `apps/web/client/remotes/{Messaging,Realtime,Channels}*.go`,
  `apps/web/client/models/{Conversation,Message,Channel,Realtime,Delivery,Author,Attachment,Block,Span,ImageRef}*.go`,
  `apps/web/client/tests/{Sidebar,Conversation,Realtime}*.go`,
  `apps/web/client/setups/{Sidebar,Conversation,Realtime,Messaging,Channels}*.go`
  (except `MessagingSend*`, which is F3's).
- **Builds:** the sidebar (the domain switcher for people with accounts in
  several domains, a "New message" button at the top of the sidebar that
  publishes F3's `SendTarget{}` to open an empty composer, channels, DMs,
  email conversations,
  unread and mention counts), the message list with live updates (bodies
  rendered only from the server's typed blocks, CONTRACTS "Rich text"), reactions,
  edit and delete, delivery status badges, sanitized email display
  (AC-IN-5).
- **Depends on:** F1, M3, R1, C2.

### F3. Web composer
- **Owns:** `apps/web/client/components/composer/**`,
  `apps/web/client/services/Composer*.go`,
  `apps/web/client/remotes/{Directory,Attachments}*.go`,
  `apps/web/client/models/{Recipient,Send,Resolve,Upload,EmailOptions}*.go`,
  `apps/web/client/tests/Composer*.go`,
  `apps/web/client/setups/{Composer,Directory,Attachments}*.go`, and the send
  call `apps/web/client/remotes/MessagingSend*.go` and
  `apps/web/client/setups/MessagingSend*.go` (an exception to F2's
  `Messaging*` pattern, agreed by F2 and F3).
- **Builds:** the one composer: recipient chips with Chat / Email / Both,
  and "Email · other domain" for a member of another hosted domain
  (AC-CP-1..5), subject, Cc, Bcc, the who-gets-what summary, attachments,
  `409 recipients_changed` handling, idempotent retry.
- **Depends on:** F1, D1, M3, A1.

### F4. Web threads, search and notifications
- **Owns:** `apps/web/client/components/{thread,search,notifications}/**`
  (the presence control lives in `components/notifications/presence/` and
  `components/notifications/presence-menu/`, not in F1's `components/app/`),
  `apps/web/client/services/{Thread,Search,Notification}*.go` (including
  `NotificationPresence*` and `NotificationPush*`),
  `apps/web/client/remotes/{Thread,Search,Notify,Presence}*.go`,
  `apps/web/client/models/{Thread,SearchHit,Notification,Presence}*.go`,
  `apps/web/client/tests/{Thread,Search,Notification,Presence}*.go`,
  `apps/web/client/setups/{Thread,Search,Notify,Notification,Presence}*.go`.
  F4 reads thread replies with its own `Thread*` remote but reuses F2's
  `Message`, `Block`, `Span` and other message models; it never copies
  them.
- **Builds:** the thread pane, search with filters (AC-SR-3), the
  notification list and push opt-in, presence dots and Do not disturb.
- **Depends on:** F1, M3, S1, N1, R1.

## Later (after the first release)

- **X1** SMTP submission for third-party mail clients (app passwords or
  SASL OAUTHBEARER).
- **X2** Port external accounts, calendar and contacts sync from
  `internal/` at commit 4ef82f72 into a Yon app.
- **X3** CADUCEUS data migration tool.
- **X4** Interface translations and in-browser message translation.
- **X5** Split `web` into separately deployed feature bundles once TACHYON
  can mount separately built Tac apps.

## Ownership check

| Path | Owner |
| --- | --- |
| `docs/**` (except `docs/mesail/ux/**`), `tests/blackbox/**`, `tools/crap/**`, `scripts/kit-sync.sh`, `CLAUDE.md`, `AGENTS.md`, `README.md`, `CHANGELOG.md`, `.gitignore` | architecture (this plan); slices append to `CHANGELOG.md` only under their own heading, and update CONTRACTS.md per rule 3 |
| `kit/<package>/**` and `apps/*/server/packages/<package>/**` | the slice that owns the package: I1 `oidc`, `identity`; M2 (from I2) `events`, `outbox`; D1 `recipients`, `directory`; C2 `channels`; A1 `blobs`, `attachments`; M1 `mime`, `mailauth`, `richtext`, `mailin`; M2 `compose`, `dkim`, `smtpclient`, `mailout`; S1 `textindex`, `search`; M3 `messaging`; R1 `realtime`; N1 `webpush`, `notify`. The workflow package of a service (rule 8b) is copied only into that service's app; `ids` and `collections` stay with architecture |
| `apps/<service>/**` other files | the service's slice |
| `apps/web/client/**` | split between F1-F4 by the patterns above |
| `apps/web-admin/**` | F5 |
| `edge/smtp/**` | M1 |
| `edge/realtime/**` | R1 |
| `Dockerfile` (non-Keycloak stages), `docker-compose.yml`, `.dockerignore`, `.env.example`, `deploy/lambda/**`, `deploy/floci/**`, `deploy/router/**`, `scripts/floci/**`, `scripts/native/**` | P1 |
| `docs/mesail/ux/**` | `mesail-ux` |
| `Dockerfile` Keycloak stages, `deploy/keycloak/**` (including the realm templates in `deploy/keycloak/realm/`) | `mesail-keycloak` |
| `.github/workflows/**`, `scripts/gates.sh`, `scripts/ci/**` | C1 |
| `scripts/vendor/**`, `tools/styles/**`, `tests/responsive/**` | F1 (C1 calls them from `scripts/gates.sh` for Tac apps) |
