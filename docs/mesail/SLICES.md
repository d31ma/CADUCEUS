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
   `MAC_HEAVY_COORDINATOR=caduceus-build mac-heavy --owner <slice> -- ...`
   (Mesail's own lock root, `~/Library/Caches/mac-locks/caduceus-build`,
   4 slots, since 2026-11-11; `tachyon-build` belongs to the TACHYON
   coordinator). Docker and VM jobs use the root `mesail-docker`
   (`TACHYON_LOCK_ROOT=~/Library/Caches/mac-locks/mesail-docker`).
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
   CRAP limit by the gate. **Or** it lives in app-local plain-Go packages
   `apps/<app>/server/packages/<package>/` (decided 2026-11-11 for slice
   M3): allowed for code that only that app uses, when the package is not
   named in `kit.list` and not under `vendor/`. `scripts/gates.sh` (slice
   C1) runs `go vet`, `go test` with coverage and the CRAP check on every
   such package, exactly as on `kit/`, so both places get the same checks.
   Code that a second app needs moves to `kit/`. Role files stay thin: a public method makes one
   call into that package and returns; a private helper holds at most one
   decision. The package talks to FYLO, HTTP clients and the clock through
   small Go interfaces that the role files fill in, so it is testable
   without Tachyon. Design this way from the start (wave 2 slices M3, R1,
   N1 included).
9. **FYLO on ATLAS (every FYLO-owning slice).** Live roots are on ATLAS in
   Lambda form, where a write takes about 109 ms with stalls up to 11 s
   (ADR 0004 section 4). So: at most 25 FYLO writes and 20 seconds of work
   per request or job call (answer `more: true`; the trigger calls again
   within its tick, EVENTS.md section 4 item 5); every write idempotent; slow work through the outbox; callers
   use a 30 s timeout and at most 3 retries with backoff. **Services on the
   sign-in path** (`directory`, which `identity` reads on every sign-in,
   refresh and switch) use a smaller budget for job and relay calls: at
   most 10 writes and 3 seconds of work per call, so a user request never
   waits long behind background work (decided 2026-11-12, slice K3).
10. **Collection names.** Build every FYLO collection name with
   `kit/collections` (`CollectionName{}.ForDomain` or `.ForDomainMonth`).
   FYLO accepts only lowercase letters, digits and `-`; the domain becomes a
   16-character hex key (DOMAIN.md, "The domain key"). Add `collections` to
   your `server/packages/kit.list`.
11. **The domain is the tenant.** Every store method takes the `domainId`
   from the verified caller (`oidc.Caller.DomainId`: the `domain` claim of
   the Mesail access token since ADR 0005; before Wave 3's L5, the Keycloak
   realm name). Chat data never crosses a domain. Email
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
16. **TACHYON 26.46.02 facts every Yon slice follows** (found by slices M3
    and R1, 2026-11-11):
    - **Outgoing HTTP without keep-alive.** TACHYON's Go prelude sends
      every outgoing request, loopback included, through its egress guard,
      and for plain HTTP the guard keeps piping a kept-alive connection to
      the first target it saw. Go reuses idle connections for any host, so
      a later request can reach the wrong service. Until TACHYON fixes it,
      every Mesail HTTP client uses `kit/transport`'s
      `SingleUseTransport{}.Client(timeout)` (a clone of the default
      transport with keep-alives off; the proxy setting stays). Add
      `transport` to your `kit.list`.
    - **Request-body fields without `,omitempty` are required** under
      `ty start` (`400 "body.<field> is required"`); `ty test` does not
      catch it. Optional fields get `,omitempty`.
    - **`@HandlesError` parameter types and `@FromBody` types are
      package-main types** (in `server/models/` or `server/shared/`). A
      kit or app-package type passes `ty test` but fails `ty start` and
      `ty bundle`.
    - **Error types go in `server/shared/`**, not `server/errors/`:
      identity providers and workers compile without `server/errors/`.
    - **Path variables match `[a-z0-9_]`** (TY2015).
    - **A public role-file method that is called only through a Go
      interface fails TY1333** in `ty bundle`: keep stores in plain Go and
      pass the `DataSource` in (rule 8b).
    - **A missing or forged token gets TACHYON's own plain-text `401`**
      ("Sign in to call this route."), not an `ApiError` body (CONTRACTS.md
      "Conventions").
    `scripts/gates.sh` bundles every app and starts every server app, so
    most of these fail the gate rather than production.
17. **Operator settings accept text.** TACHYON 26.46.02 passes an
    environment override to a Yon setting as a JSON string (`"true"`,
    `"60"`), whatever the setting's type. So every setting that
    `docs/mesail/OPERATIONS.md` lists, or that an operator may set from an
    env file, is read by a settings reader that accepts both the JSON
    value and its text form (`true` and `"true"`, `60` and `"60"`), and
    refuses any other text at start with a clear message (decided
    2026-11-11, from slice OP1; a TACHYON request asks for typed
    overrides).
18. **Thin controllers and numbered errors** (the owner's rule of
    2026-11-12, ADR 0009). A controller route method has exactly one
    statement: it returns the result of one method of one Service
    (`return c.messages.Post(request, body)`). Every backend app has
    `server/services/`; finding the caller, checks, branches, loops and
    wire mapping live in the Service or below it. Every error answer that
    Mesail code shapes has `ApiError.message` =
    `[MS<4 digits>]: <plain English message>`, with the number from the
    app's block in `docs/mesail/ERRORS.md`, built by the app's
    `@ErrorHandler` with `kit/apierrors`. New code follows this rule from
    the day E0 merges; existing apps change in Wave 7.

## Gates

Every slice runs, in its worktree, before it merges:

```sh
scripts/gates.sh --app <each app it owns>   # versions, doctor, gofmt, kit sync, every Go module outside apps/ (gofmt, vet, test, CRAP), ty migrate check, ty test --coverage, ty bundle, ty start
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

**Every Go module is gated** (decided 2026-11-11, after a router fix
merged without a gate). `scripts/gates.sh` runs `gofmt -l`, `go vet ./...`
and `go test ./...` in every Go module of the repository outside the apps:
`kit`, `deploy/router`, `edge/realtime`, `edge/smtp`, `tools/crap`,
`tools/styles`, `tests/responsive` and `tests/blackbox`. `go vet` uses the
build tags the module's files declare (`blackbox`, `responsive`), so the
tagged suites are vetted; `go test` runs without tags, so the blackbox
suite (Floci) and the responsive suite (after the web bundle) run in their
own steps. The production modules (every module outside `tests/` and
`tools/`: today `kit`, `deploy/router`, `edge/realtime`, `edge/smtp`) also
pass the CRAP check (at most 15). A new `go.mod` outside `apps/` is gated
automatically: the script finds the modules instead of listing them. A slice that owns one of these modules
runs `scripts/gates.sh --shared-only` before it merges. Slice C1 owns the
script change.

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
> Changed by Wave 3 (ADR 0005): one Keycloak realm, `mesail`; see L1, L2 and L4.

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
> Changed by Wave 3 (ADR 0005): one Keycloak realm, `mesail`; see L3.

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
> Changed by Wave 3 (ADR 0005): one Keycloak realm, `mesail`; see L6.

- **Owns:** `apps/web/client/manifest.json`, `apps/web/client/pages/**`,
  `apps/web/client/components/app/**`, `apps/web/client/shared/**`,
  `apps/web/client/services/Shell*.go`, `apps/web/client/services/Session*.go`,
  `apps/web/client/remotes/Identity*.go`, `apps/web/client/models/{Session,SignIn,Refresh,Me}*.go`,
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
> Changed by Wave 3 (ADR 0005): one Keycloak realm, `mesail`; see L7.

- **Owns:** `apps/web-admin/**` (it runs F1's `scripts/vendor/duvay.sh
  --app web-admin` for its own vendor copy).
- **Builds:** domains (with DNS record display and verify, for operators),
  members and aliases, channel email addresses (SPEC S11, AC-DM-1, AC-CH).
- **Depends on:** D1, C2 contracts.

### P1. Lambda images and the Floci stack
> Changed by Wave 3 (ADR 0005): one Keycloak realm, `mesail`; see L5.

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
  volumes (the owner's decision), Lambda timeouts of 120 s (60 s since
  K8), an entrypoint
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
  Push sender (`kit/webpush`, VAPID keys from the secret store); the
  browser side comes with slice N2 (Wave 3b) once Mesail uses a TACHYON
  release with the push client (TACHYON ADR 0118).
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

## Wave 3: one login (ADR 0005)

The owner's requirement of 2026-11-11: one login per person, with the
email prefix as the username in every domain (SPEC AC-ID-1 to AC-ID-14,
S12, S14; ADR 0005; CONTRACTS "identity" and "directory"). It replaces the
realm-per-domain model of I1, D1, F1, F5, P1 and `mesail-keycloak`. Where
an earlier slice text in this file says "realm of the domain",
"`mesail-operators`" or "creates its Keycloak realm", this wave wins.

### What changes where

| Area | Change |
| --- | --- |
| Keycloak (`deploy/keycloak/**`) | One realm `mesail` instead of one per domain plus `mesail-operators`; username = prefix; clients `mesail-identity`, `mesail-account`, `mesail-directory` (in the realm `mesail`); no `mesail-service` client; realm role `mesail-operator` only; user attribute `mesail.mustChangePassword` mapped into the `mesail-identity` token; dev users by prefix. `domain-realm.json` and `mesail-operators-realm.json` are deleted. |
| `kit/oidc` | Mesail token mode: exact issuer, `domain` claim -> `Caller.DomainId`, `oidc.InstallWide`; a token signer (ES256) and key set document for `identity`; `TokenRequest.Domain` for tests; `ServiceTokens` calls `identity`'s service token route; the session code signs in to the one realm by username. `Realm` and `IssuerPattern` are removed in L5. |
| `kit/addresses` (new) | Prefix rules, login parsing (prefix or address), address normalization, the role names (SPEC AC-ID-11). Used by identity, directory, mail-in, messaging. |
| `apps/identity` | Sign-in by prefix or address, the person lookup, Mesail token issuing, `GET /jwks`, `POST /service-token`, `POST /session/domain`, membership check on refresh, `mustChangePassword`, account routes in the realm `mesail`. |
| `apps/directory` | `people` and `local-parts`, member admin routes, person routes, the AC-ID-13 check, default domain, Keycloak users through `mesail-directory`; no realm creation (`packages/keycloak` realm template code and `RealmService` go away); prefix check on aliases and channel addresses; `member.upserted` with `removed`. |
| Every Yon app | `application.json` `oidc` block (issuer, `jwks`, `servicetokens`; no `keycloak`), the kit copy of `oidc`, test token setups (`Domain` field), install-wide calls with `oidc.InstallWide`. |
| `kit/realtime`, `kit/channels`, `kit/mailin`, `kit/mailout` | `OidcTokenChecker` reads the `domain` claim; `"mesail-operators"` becomes `oidc.InstallWide`. |
| P1 (`deploy/floci/**`, `deploy/lambda/**`, `deploy/router/**`, `scripts/floci/**`, `scripts/native/**`, `.env.example`) | `stack.json` `keycloak.domains` and `testUsers` become people with memberships; the signing key and the service secret in `generatedSecrets`; `rewrite-config.sh` rewrites `issuer`, `jwks` and `servicetokens` to `identity`'s URL; `tokens.sh` gets tokens from `identity`'s sign-in, not from Keycloak; `keycloak.sh` checks the realm `mesail`; dev seed adds memberships. |
| `apps/web` | Sign-in field "Username or email address"; no domain guessing (`SessionRealmService` goes away); one `Session` per tab; domain switcher from `Session.domains`; `mustChangePassword` screen; default domain in settings. |
| `apps/web-admin` | Operator signs in with the same form; members page (add by prefix, existing-person notice, admin flag, remove, last admin); people page for Operators (find, rename, disable, reset password, remove 2-step code, delete); name the first admin on the domain page. |
| `docs/mesail/ux/**` | Screens for the sign-in field, the switcher, the members and people pages. |
| `tests/blackbox`, `tests/responsive` | New tests for AC-ID-1 to AC-ID-14; fixtures by prefix; no realm names. |

### LU. UX for one login (`mesail-ux`)
- **Owns:** `docs/mesail/ux/**`.
- **Builds:** the sign-in screen with one "Username or email address"
  field and the `403 not_a_member` / `no_domain` messages; the domain
  switcher fed by `Session.domains` (per tab, admin badge); the forced
  password change screen; the default domain setting; the admin members
  page (add by prefix with the "existing person" notice, admin flag,
  remove, `409 last_admin`) and the Operator people page. Every screen at
  every width (rule 15).
- **Depends on:** nothing; starts at once.
- **Done when:** L6 and L7 can build from it.

### L0. Blackbox tests for one login (architecture, `mesail-arch`)
- **Owns:** `tests/blackbox/**`.
- **Builds:** throwaway contract tests for SPEC AC-ID-1 to AC-ID-14 (sign
  in with `ana`, `ANA` and `ana@example.org`; switch; refresh after
  removal; prefix reservation against aliases and channel addresses;
  last admin; AC-ID-13; install-wide `*`; the key set and the service
  token route), and fixtures by prefix (ana: admin of `example.com` and
  member of `example.org`; bo: `example.com`; eve: `example.org`; carol:
  `partner.test`; operator: no domain).
- **Depends on:** nothing; starts at once. The tests fail until L5 lands.
- **Done when:** `go vet -tags blackbox ./...` passes and every new
  criterion has a test. Landed: `SignIn_test.go`, `DomainSwitch_test.go`,
  `Tokens_test.go`, `Members_test.go`, `People_test.go` (helpers
  `Sessions.go`, `People.go`; fixtures in `tests/blackbox/README.md`).

### L1. Login kit (new session `mesail-login-kit`, or I1's successor)
- **Owns:** `kit/oidc/**`, `kit/addresses/**` (new). Not the app copies:
  L4 and L5 sync them.
- **Builds:** `kit/addresses` (SPEC AC-ID-11: `Username`, `Login` parse
  into username and optional domain, `Address` normalize and compare, the
  role names). In `kit/oidc`: the Mesail token mode of `Gate` and
  `TokenVerifier` (exact issuer, `domain` claim, `azp` allowlist
  `mesail-web`/`mesail-service`, `InstallWide`), chosen when `oidc.issuer`
  has no `{realm}`, so apps keep working in the old mode until L5; a
  `TokenSigner` (ES256, current and previous key, `kid`) and the key set
  document for `identity`; `TokenRequest.Domain` in `SigningRealm`;
  `ServiceTokens` against `oidc.servicetokens` with the per-domain cache;
  the session and account code for the one realm (username, not address).
- **Depends on:** nothing; starts at once.
- **Done when:** both modes pass their tests at 100% of the new code, and
  the CRAP and cohesion gates pass.

### L2. One Keycloak realm (`mesail-keycloak`)
- **Owns:** `deploy/keycloak/**`, the Keycloak stages of `Dockerfile`.
- **Builds:** `deploy/keycloak/realm/mesail-realm.json` per the "What
  changes where" table: brute-force detection on, `loginWithEmailAllowed`
  false, `editUsernameAllowed` false, `duplicateEmailsAllowed` true,
  registration off, 60 s access tokens, conditional OTP on the direct
  grant flow, the three clients with their service-account roles, the
  attribute mapper; dev users `ana`, `bo`, `eve`, `carol`, `operator`
  (role `mesail-operator`); the OTP provider endpoint works in the realm
  `mesail`; `test/oidc_check.py` checks the new realm. Delete
  `domain-realm.json`, `mesail-operators-realm.json` and the per-realm dev
  user files.
- **Depends on:** nothing; starts at once.
- **Done when:** the image starts on SQLite with the one realm, and
  `oidc_check.py` passes direct grant, OTP, attribute mapping and the
  three service accounts.
- **Follow-up (2026-11-11, from the Floci run):** the dev and test users in
  `deploy/keycloak/realm/` carry fixed Keycloak user ids (`id` in the
  realm import), and the directory's dev seed (L3, P1's people list) uses
  the same ids, so a Keycloak that is re-created for a run keeps every
  `sub` and the tests' member ids still match (CONTRACTS "identity", a
  Keycloak user without a person). A fresh FYLO root per Floci run stays
  allowed, but is not needed for this.

### L3. Directory: people and members (D1's successor)
- **Owns:** `apps/directory/**` (except kit copies), `kit/recipients/**`.
- **Builds:** CONTRACTS "directory" members and people routes; the
  `people` and `local-parts` collections (DOMAIN.md) with the write order;
  the AC-ID-13 check; the prefix check on aliases and channel addresses;
  Keycloak user create, disable, delete, password reset, OTP removal and
  `mesail.mustChangePassword` through `mesail-directory` (a fake Keycloak
  in tests); `GET /people/{personId}` for `identity`; the default domain
  route; `member.upserted` with `removed` into each affected domain;
  domain removal drops memberships. Removes the realm creation
  (`packages/keycloak` realm code, `RealmService`, the realm secrets).
  Seeds a dev person store from P1's people list.
- **Depends on:** L1 (`kit/addresses`, `InstallWide`); fakes until L2.
- **Blackbox:** the L0 member and prefix tests.
- **Done when:** the app bundles, starts and answers `GET /health` 200,
  and its routes pass against the L2 image.

### L4. Identity: one sign-in, Mesail tokens (I1's successor)
- **Owns:** `apps/identity/**` (except other slices' kit copies).
- **Builds:** CONTRACTS "identity": sign-in by prefix or address, the
  person lookup, the `403` answers, the Mesail token with the claims
  listed there, `GET /jwks`, `POST /service-token`, `POST
  /session/domain`, the membership check on refresh, `mustChangePassword`,
  the account routes in the realm `mesail`; `names.words` for the JWK
  names. Settings `identity.issuer` (not `token.issuer`: `ty` refuses an
  `application.json` key named `token`, TY2010) and the env names in
  CONTRACTS.
- **Depends on:** L1; L3's `GET /people/{personId}` (fake until it lands);
  L2 for the live check.
- **Blackbox:** the L0 sign-in, switch, refresh, key set and service
  token tests.
- **Done when:** the app bundles, starts and answers `GET /health` 200,
  and a sign-in against the L2 image returns a Mesail token that L1's
  `Gate` accepts.

### L5. Switch every app to Mesail tokens (P1)
- **Owns, for one commit:** the `oidc` block of every
  `apps/*/server/application.json`; every `apps/*/server/packages/oidc/**`
  copy (through `scripts/kit-sync.sh`); the token setup files of each
  app's tests (`*TestSetup.go`, `TokenIdentityProviderTest.go`,
  `CallerServiceTest.go` and the others that build tokens); the
  install-wide call sites (`kit/realtime/OidcTokenChecker.go`,
  `SweepTrigger.go`, `Sweeper.go`, `kit/channels/ScheduledSweep.go`,
  `kit/mailin/Directory.go`, `kit/mailout/Transport.go`,
  `apps/directory/server/workers/outbox-relay.go`,
  `apps/directory/server/services/CallerService.go`,
  `apps/notify/server/services/CallerService.go`,
  `apps/notify/server/workers/notify-jobs.go`,
  `apps/mail-out/server/workers/deliver.go`,
  `apps/messaging/server/controllers/OutboxController.go` and their
  tests); and, as always, P1's own files.
- **Builds:** the switch of the whole install in one fast-forward: every
  app in the Mesail token mode, `"mesail-operators"` replaced by
  `oidc.InstallWide`, the old mode removed from `kit/oidc` (`Realm`,
  `IssuerPattern`, the `{realm}` settings), the P1 changes in the "What
  changes where" table.
- **Keycloak transition (from L2, develop 345a816):** while
  `MESAIL_KEYCLOAK_DOMAINS` is set, the Keycloak image still imports the
  old realms beside `mesail`. L5 stops setting it (`docker-compose.yml`,
  `scripts/floci/up.sh`, the native scripts); then `mesail-keycloak`
  deletes the transition files (see
  `~/Library/Logs/local/CADUCEUS/mesail/keycloak-l2-note.md`).
- **Depends on:** L1, L2, L3, L4 on `develop`. The coordinator asks the
  owners of the apps listed above to merge nothing during the switch
  (about one working session).
- **Blackbox:** all of L0.
- **Done when:** `scripts/gates.sh` passes for every app, the native stack
  and the Floci stack start, and the L0 tests pass against the running
  stack. This is also L3's live check: L3 tested its Keycloak calls only
  against a fake, so L5 runs `directory`'s member and person routes against
  the L2 image (create, disable, delete, password reset, 2-step code
  removal, `mesail.mustChangePassword`) and D2's recovery of a Keycloak
  user left by a failed add (`mesail.addRequest`, once `mesail-keycloak`
  declares the attribute), and reports any gap to the directory owner.

### L6. Web: sign-in and domain switcher (F1's successor)
- **Owns:** F1's patterns in `apps/web/client/**` (`Session*`, `Shell*`,
  `SignIn*`, `DomainSwitch*`, `Settings*`), `SessionRealmService` (deleted).
- **Builds:** the LU screens: one sign-in field, the `403` messages, the
  per-tab session with `Session.domains`, the switcher (`POST
  /session/domain`, then a new realtime connection), the forced password
  change, the default domain setting (`PUT /directory/me/default-domain`).
- **Depends on:** LU and the contracts; works with fakes until L4 lands.
- **Also owns:** `tests/responsive/AppProfile.go` (`WebProfile`): the
  stored session and fake tokens follow the new per-tab session.
- **Done when:** gates pass, the responsive test passes at 320 to 2560 px,
  and a person switches domains in the running stack after L5.

### L7. Admin app: members and people (F5's successor)
- **Owns:** `apps/web-admin/**`.
- **Builds:** the LU admin screens: Operator sign-in with the same form
  (no `mesail-operators` realm), the members page, the people page, naming
  the first admin on the domain page.
- **Depends on:** LU and the contracts; works with fakes until L3 lands.
- **Also owns:** `AdminProfile` in `tests/responsive/AppProfile.go` (no
  `mesail-operators` realm; L6 and L7 merge it one after the other).
- **Done when:** gates pass, the responsive test passes, and S14 works in
  the running stack after L5.

### Order

1. At once, in parallel: **LU**, **L0**, **L1**, **L2**.
2. After L1: **L3** and **L4** in parallel; **L6** and **L7** may start on
   the contracts with fakes (after LU).
3. After L1 to L4 are on `develop`: **L5**, alone, with a merge pause for
   the apps it touches.
4. After L5: L6 and L7 finish their live checks; L0's tests pass against
   the stack.
5. After L3, any time (does not block L5): **D2** (merged, develop
   f106bdb).
6. After D2, any time (does not block L5): **L7b**.
7. After L5 (merged, develop b3dbc95), any time: **K1**.
8. Any time: **K2** (it may change `deploy/keycloak`, so it goes through
   `mesail-keycloak` for that part). K2 merges its Keycloak fix on its own.
9. After K2, and not at the same time as OP2 (both change `deploy/router`):
   **K3**.
10. Any time: **K4**.
11. Any time, not at the same time as K4 (both change `kit/events`):
   **K5**.
12. After K3: **K6** (with OP2 if OP2 is changing `deploy/lambda`).
13. After K4: **K7**.
14. Now, before K1's Floci runs if they have not started (it needs the
    dev host; K3's series stopped): **K8**. Floci series are not reliable
    until K8 finds the store outage's cause.
15. After K5: **K9**.
16. After K8 and the next TACHYON release (the owner approves it): **K10**.

### K5. Channel events without reader lists (messaging's, realtime's and notify's owners, one session)
Found by K1 (2026-11-12): `memberIds` in a channel message's events lists
every reader, so a channel of about 5,000 members passes the 512 KiB
document cap (in `messaging`'s staged event and `notify`'s spilled fan-out
record). Decided in EVENTS.md section 7, "Reader lists"; no channel size
limit and no owner question.
- **Owns, for this change only:** `kit/events` payload comments and tests,
  `messaging`'s event staging, `realtime`'s fan-out (with R1's successor),
  `notify`'s fan-out (with N1's successor), and the "Open" item about
  channel size that K1 wrote in DOMAIN.md (replace it with a pointer to
  EVENTS.md "Reader lists").
- **Builds:** `messaging` stages channel events with empty `memberIds` and
  direct ones with the participants (at most 100), and caps
  `mentionedMemberIds` at 100; `realtime` fans out channel events from its
  own channel membership copy (kept from `channel.member.*`; add it if it
  does not exist yet) and reads `memberIds` only for direct ones; `notify`
  uses `mentionedMemberIds`, thread participants and direct participants
  only; a direct conversation refuses a 101st member participant (`400
  invalid_request`, field `recipients`).
- **Done when** (changed 2026-11-12: seeding 10,000 participants in real
  FYLO takes about 100 seconds, over `ty test`'s fixed 30 seconds per file):
  - a real-FYLO `ty test` in `apps/messaging` with a channel of 1,000
    members: a post with a mention, an edit and a reaction; the message,
    conversation and reaction documents stay under 512 KiB and the staged
    events have empty `memberIds`;
  - a Go test in `apps/messaging`'s packages (in-memory stores) with
    10,000 members and the same checks;
  - a `kit/events` test: the largest direct event (100 members, 100
    mentions) stays under 16 KiB;
  - the realtime and notify tests show channel readers still get the
    message and mentioned members still get a notification; gates pass.

### K9. A channel post reads no member list (messaging's owner, after K5)
Found by K5 (2026-11-12): a post in a channel makes `messaging` read every
participant row of the channel (`ConversationPicker.channel` calls
`Participants.OfConversation`, for mentions and the delivery plan). At
about 110 ms per page of 100 rows, a channel of 10,000 members needs about
11 seconds per post, over the client's 10 second timeout. Since K5 the
channel's readers come from `realtime`'s and `notify`'s own membership
copies, so `messaging` does not need the list. No owner question.
- **Owns:** `apps/messaging` posting, mention and delivery-plan code (with
  the messaging owner).
- **Builds:** for a channel, `messaging` checks the poster's membership
  and each mentioned member (at most 100) with one exact-match read each
  (`conversationId` and `memberId`), and reads only the rows the delivery
  plan needs that are not plain members (for example outside email
  participants, with an exact-match filter); it never scrolls all
  participants of a channel on the post path. Direct conversations keep
  reading their participants (at most 100).
- **Depends on:** K5 merged (both change `messaging`'s posting).
- **Done when:** a real-FYLO measurement shows a post's reads do not grow
  with the channel's size (the same number of reads at 100 and at 1,000
  members); a post in a 1,000-member channel answers in under 1 second in
  that test; mentions and notifications still work; gates pass.

### K4. A newer event replaces a waiting one (the outbox owner, M2's successor)
Found by K1 (2026-11-12). Decided in EVENTS.md section 2.2; no owner
question.
- **Owns:** `kit/outbox/**`, `kit/events/**` (the replace key per type),
  their app copies through `scripts/kit-sync.sh`; and in `apps/messaging`
  only the test that proves it.
- **Builds:** the replace key in `kit/events` (`message.edited`,
  `reaction.changed`: message id; `presence.changed`: member id);
  `kit/outbox` replacing a waiting event (and a dead letter) with the same
  type and key on the same document, with the union of subscribers;
  `message.deleted` dropping waiting `message.edited` of that message; a
  refusal (`503`) when staging would pass the document's 512 KiB cap.
- **Done when** (changed 2026-11-12: TACHYON 26.46.02 runs each `ty test`
  file as one call with a fixed 30 second deadline, and one real-FYLO edit
  takes about 30 ms, so 1,000 edits cannot fit):
  - a real-FYLO `ty test` in `apps/messaging` edits one message 300 times
    with a 4 KB body while `search` is down (1.2 MB of bodies, so without
    K4 the document would pass the cap after about 120 edits); the
    message document stays under 512 KiB with one waiting
    `message.edited`, and when `search` comes back it gets the newest body
    once;
  - a `kit/outbox` `go test` stages 1,000 edits of a 10 KB body and keeps
    one waiting edit, with the staged events under 16 KiB;
  - gates pass for every app that syncs the kit.

### K3. User requests before background work (a small session, built)
Found by K2 on Floci (2026-11-12, run `k2-signin/floci-20261111-232350`):
`identity`'s `GET /directory/people/{personId}` waited 10 seconds behind
`directory`'s single writer, which was busy with background calls, so the
router answered `502` and sign-in answered `503` after 13 seconds.
Decided (no owner question): user requests go first, background calls get
smaller, and `identity` retries once. A stale cache of person records in
`identity` was rejected: it would let a removed member keep a domain for
longer than SPEC AC-ID-4's 60 seconds.
- **Owns, for this change only:** `deploy/router` (the single-writer
  queue; coordinate with OP2), `apps/directory` job and relay budgets
  (with the directory owner), `kit/oidc` `IdentityPeople` or the code that
  calls `GET /people` (with L1's owner).
- **Builds:**
  - **Router queue order.** For each single-writer function, the router
    keeps two queues: user requests first, then background calls
    (`/events`, `/outbox/relay`, `/outbox/dead/requeue`, `/jobs/...`).
    When the function is free, a waiting user request always goes next. A
    background call that waits more than 60 seconds goes next once, so
    background work never starves. Event pushes to a busy writer still get
    `429` at once (the floci-502 rule).
  - **Smaller background calls in `directory`** (rule 9, sign-in path): its
    removal job, outbox relay and any other job call do at most 10 writes
    and 3 seconds of work per call, and answer `more: true`.
  - **`identity` retries `GET /people` once:** on a timeout, `502` or `503`,
    it waits 200 ms and tries again; the two tries together stay under 12
    seconds, and a sign-in that still fails answers `503` as now. The
    Keycloak calls keep their own timeout (K2).
- **Depends on:** K2 merged; not at the same time as OP2.
- **Also watch:** OP1's Floci run saw one `channels` `503` at 04:02:32 UTC
  with no slow router call and no known cause. If K3's runs see it again,
  keep the `channels` function's own log for that minute and report it.
- **Done when:** a Floci test that keeps `directory` busy with a removal of
  200 members while signing in, in a loop, shows every sign-in answered in
  under 5 seconds, and the K2 check passes 10 runs in a row.
- **Merge bar** (decided 2026-11-12, after run 1 of 10 failed for causes
  outside K3; findings in `slice-k3-floci-findings.md`):
  1. Before the series, wait until no domain is `removing` in the dev
     host's FYLO data (the two 200-member test domains from attempt 1 and
     run 0); do not remove dev host volumes. Start the series warm: every
     function answers its health route and one sign-in has passed.
  2. The K3 test has passed once (run 0, 497fde5, the same product code as
     2d12d99); keep that log. The 10-run series runs the K2 check and the
     other blackbox tests without the K3 test, so it leaves no new
     200-member removal behind. If K3's product code changes after run 0,
     run the K3 test once more after the series.
  3. K3 merges when the K2 check passes 10 runs in a row, with no
     `identity` `503` and every sign-in under 5 seconds. Another failing
     test does not block the merge only when (a) it fails the same way on
     `develop` in the same setup (an earlier log is enough), or (b) the
     logs show that test leftovers caused it. Write each such failure in
     `slice-k3-floci-findings.md` with the slice that owns it: slow `POST
     /channels` is K7, removal throughput is K6, a cold start is OP2.
  4. A failure that K3's own changes can cause (the router queue order,
     `directory`'s job budgets, `identity`'s retry) always blocks.
  5. A directory store outage (decided 2026-11-12, after run 2 had 11
     `identity` `503`s at 07:29:39-43 UTC): a run is set aside, neither a
     pass nor a failure, when `directory` itself answered `503` "the
     directory store is not available" for the whole window of the
     `503`s, and K3's parts worked as designed (no queue wait, one retry).
     The count is not restarted: K3 needs 10 passing runs, and set-aside
     runs do not break the chain. Each one is written in the findings
     note under K8. If 2 or more further runs of the series are set aside,
     the series stops and the coordinator reports to the owner.
  6. Changed after run 3 (2026-11-12): the store outage grew (47 seconds,
     `directory` and `messaging`), so more runs on this data measure the
     store, not K3. The series stops. K3 merges on its evidence, after its
     normal gates: run 0 (the K3 test passed), the warm-up and run 1
     (passed fully, slowest sign-in 2.4 seconds), and runs 2 and 3, where
     K3's parts worked as designed (no queue wait, one retry). The K2
     check of 10 runs in a row moves to K8's done-when.

### K8. Store outage on Floci: one FYLO process per root (directory's and messaging's owners, with the deploy files and TACHYON)
Found by K3 (2026-11-12, run 2): from 07:29:31 to 07:29:43 UTC every
`directory` call answered `503` "the directory store is not available"
(`DirectoryErrorHandler.Store`, a `YonResourceError` from TACHYON's FYLO
datasource). A `POST /members` hung 8.5 seconds first, then every call
failed in about 7 ms, and `directory` recovered at 07:29:44. The load was
15, and the dev host supervisor logged nothing. The same message was seen
before K3, in slice P1's Floci runs. Sign-in depends on `directory`, so
12 seconds of outage meant 12 seconds of failed sign-ins.
Run 3 (07:40:36-07:41:23 UTC, about 47 seconds, load 6 to 13) was wider:
`directory` ("the directory store is not available", 23 calls) and
`messaging` ("the message store is not available", 19 calls) failed at the
same time, mostly on writes, while reads kept answering `200`. Ruled out
by K3: function restarts, guest kernel I/O errors, host NFS messages, a
full disk (the data disk was 1% used). So K8 covers every FYLO-owning
service, not only `directory`.
**FYLO's report** (2026-11-12, FYLO coordinator's notes
`mesail-store-outage-report.md`; probes in
`~/Library/Logs/local/FYLO/mesail-store-outage/`):
- K3's index-size lead is ruled out: index files may grow to 256 MiB;
  FYLO's 1 MiB limit is only for one request line on standard input.
  FYLO measured 3.4 MB index files with no errors.
- Likely cause, not yet proven on the dev host: every `fylo exec --loop`
  takes the root's exclusive lease on its first request and keeps it until
  the process exits (FYLO ADR 0003). In Floci each container of a function
  runs its own `ty start` and its own loop on `/mnt/data/<service>`. When a
  second container of the same function runs (a replacement after a
  timeout, an old container still alive after a redeploy, or a scale-out),
  its FYLO answers `EROOTLOCKED` in about 0.1 ms to every call until the
  first container's process exits. TACHYON reports that as "FYLO operation
  failed (EROOTLOCKED)" (diagnostic 2160), and Mesail answers `503`. That
  matches the router log: one slow call (8.5, 5.5 and 6.8 seconds, a cold
  start), then fast `503`s.
**Decision (architecture, 2026-11-12; no owner question):** Mesail keeps
its design, one live process per FYLO root (ADR 0004 section 4: one
function per root with reserved concurrency 1), and makes Floci keep it:
fix option (a). TACHYON is asked for option (b) as a second line of
defence. Option (c), FYLO releasing the lease after each request, is the
FYLO owner's design choice; Mesail does not need it.
- **Owns:** `apps/directory` and `apps/messaging` store error handling
  (with their owners); for this change only, the parts of
  `deploy/floci/**` and `deploy/lambda/**` that create, replace and stop
  function containers and set their timeouts (OP2 owns `deploy/**`:
  coordinate if OP2 is running); any TACHYON gap goes through the
  coordinator.
- **Builds:**
  - **Prove it first** on the dev host: during an outage, `docker ps`
    shows two `floci-aws-mesail-<service>-*` containers of one function,
    Floci's log is kept through the window, and the FYLO error behind each
    `503` is `EROOTLOCKED` (through `YonLog` below, or a direct probe of
    the datasource error). Write the proof in the findings note.
  - **Fix (a), one container per FYLO root at any time, in Floci:**
    - every FYLO-owning function's timeout in `stack.json` is 60 seconds
      (it was 30, and 60 for `attachments`, `mail-in` and `mail-out`),
      under the router's 70 second upstream timeout
      (`MESAIL_ROUTER_TIMEOUT_SECONDS`); ADR 0004 section 4 says 60
      (decided 2026-11-12: 20 seconds of work, an ATLAS stall of up to 11
      seconds and a cold start fit, and a stuck container is replaced
      sooner than at 120);
    - a redeploy stops the old container before the new one takes calls;
    - a deploy guard during Floci runs checks every 5 seconds for two
      containers of one FYLO-owning function, stops the older one, and
      logs it;
  - **Second lead, from K8 (2026-11-12): two `fylo` processes in one
    container.** K3's Floci logs show no second container in either
    window, and Floci 2.4.0 honours reserved concurrency 1. TACHYON
    v26.46.02's data source gives each store call a 5 second deadline; when
    it fires, the `fylo exec --loop` process is killed without a wait and
    the next call starts a new one. A process stuck in a slow disk write
    (ATLAS stalls up to 10.9 seconds) cannot die until the write ends, so
    it keeps the root lease, and the new process gets `EROOTLOCKED`. That
    fits the router log (the first `503` after 5.5 to 8.5 seconds, then
    fast `503`s, two services at once on one shared disk), and TACHYON's 2
    second wait (its ADR 0128) does not cover a longer stall. K8 proves it
    with a read-only guest watcher (fylo processes per container, their
    state and kernel wait, the disk's in-flight counters) under K3's load.
    A controlled 10 second `fsfreeze` of the guest data disk needs the
    coordinator's OK, and only after K8 has checked that the dev host
    supervisor's write probe will not block the host for it. If proven,
    TACHYON is asked (through the coordinator) to wait for the old `fylo`
    process to exit, for a bounded time, before it starts a new one;
    - `directory` and `messaging` (and every FYLO-owning service through
      its store error handler) report `EROOTLOCKED` as "store busy" in
      their log, not only "store is not available", so the next outage
      names itself.
  - **Ask TACHYON for (b)** through the coordinator: in Lambda form, use
    FYLO's per-invocation lease (FYLO ADR 0011, a bounded wait for the
    root) instead of one long-lived loop per container, or at least treat
    `EROOTLOCKED` as busy and wait a bounded time (about 2 seconds) before
    failing. This also covers AWS Lambda later, where an old environment
    can stay alive after a redeploy.
    **TACHYON's answer** (TACHYON `develop` 3a1008a0e, TACHYON ADR 0128,
    not released yet): a failed store call writes `handler.storage_failed`
    with the `request_id` and `TY2160` "FYLO operation failed (<code>)" to
    `ty`'s standard error (no data, token or path); on `EROOTLOCKED` the
    data source retries for up to 2 seconds (10 ms, doubling to 200 ms)
    before it fails. The per-call lease is only a plan: FYLO ADR 0011's
    lease is a Rust library API, not in the `fylo` binary, and not approved
    for deployment. So fix (a) above is the real fix; after Mesail's
    TACHYON pin moves, the outage names itself and short overlaps of two
    containers end within 2 seconds.
  - make the cause visible first, by using TACHYON's running handler-log
    work, not a new request: session "TACHYON: Yon handlers can write logs
    on every answer" (branch `feat/yon-handler-logs`, 2eade57a5, its ADR
    "Yon handlers write logs on every answer"; not merged or released
    yet). With it, `directory`'s store error handler calls
    `YonLog.Error("...", fields)` with what the `YonResourceError`
    carries, and the line reaches TACHYON's run log. It does not cover two
    parts, which the coordinator passes to TACHYON's coordinator: (1)
    TACHYON's FYLO datasource writing its own cause (the FYLO error behind
    the `YonResourceError`) to the run log, because the handler sees only
    what the error carries; (2) a statement or test that the run log
    reaches the Floci function log in Lambda form (the ADR does not
    mention Lambda). Mesail uses it after the TACHYON release that has it
    and the pin move; until then, use the router, Floci and dev host logs.
    TACHYON answered part (2) on 2026-11-12: handler log lines go only to
    `ty`'s standard error, `deploy/lambda/start.sh` runs `exec ty start`
    with no redirect, so on real Lambda they reach CloudWatch; Floci is a
    known limit of their ADR. So K8 checks once, with Floci's logs API,
    that a line `ty` writes to its standard error appears in the function
    log, and writes the result in the hand-over. A successful answer has
    no `x-tachyon-request-id`, so its access line has no `request_id`;
    failures carry the id, and all lines of one call share it. K8 does not
    ask TACHYON for the header on successful answers: the outage is
    failures, and the router's log gives the time of each call;
  - if the proof fails (one container only during an outage), look at the
    other causes with logs: an ATLAS write stall (ADR 0004 section 4:
    stalls up to 11 seconds), the FYLO process restarting, or the
    datasource marking the store down after one slow call;
  - do not delete or reset the dev host's data to make runs pass before
    the cause is known.
- **Not now:** a longer `identity` retry (it would hold sign-in past the
  client's 10 second timeout). A later option, only if outages remain
  after this slice: `identity` keeps each person record for at most 60
  seconds and uses it only when `directory` answers `503`; that keeps SPEC
  AC-ID-4's 60 second limit. It needs an architecture decision first.
- **Depends on:** the dev host, so after K3's and K1's Floci runs; the
  `YonLog` part after the TACHYON release and pin move.
- **Done when:** the cause is written in the hand-over with logs; the K2
  sign-in check passes 10 runs in a row on Floci (moved here from K3; a
  failure that points at K3's parts comes back as a fix slice); and a
  1-hour Floci run under K3's sign-in load shows no "store is not
  available" answer, or the owner accepts a written limit.
- **Merge, changed 2026-11-12:** TACHYON's fix (its `develop` ff94a61dd,
  an amendment to its ADR 0128; not released) makes the next store call
  wait, within its own deadline, for a killed `fylo` to exit, logs
  `storage.fylo_process_stopped`, and adds the data source setting
  `datasource.operationTimeoutMs` (1,000 to 60,000, default 5,000). The
  Floci image installs `ty` only from a release, so the rerun needs one.
  K8 merges its own parts now (the 60 second function timeout and the
  writer guard, with the proof in the findings note); the K2 10-run check
  and the 1-hour run move to K10.

### K10. Move to the next TACHYON release (a small session, after K8 and that release)
- **Owns:** the TACHYON version and SHA-256 pins (the Floci image, the
  native setup and CI), the FYLO-owning apps' `application.json` data
  source settings, and their store error handlers' logging.
- **Builds:**
  - the pin moves to the first TACHYON release with ADR 0128 and its
    amendment (ff94a61dd), after the owner approves that release;
  - `datasource.operationTimeoutMs` 15,000 for every FYLO-owning service
    in Lambda form (the dev host, kestrel and AWS later); native runs keep
    5,000 (decided 2026-11-12: ATLAS write stalls reach 10.9 seconds and
    cold-start writes took 9.5 and 18 seconds, so 5 seconds turns them
    into failures; a shorter deadline does not free the store sooner,
    because the next call waits for the stuck `fylo` to exit anyway;
    `directory` gets the same value). Since the owner's approval of
    2026-11-12, TACHYON's budget is for ALL store calls of one HTTP
    request together (the wait for an old `fylo` and the `EROOTLOCKED`
    wait included), not per call. Checked: 15 seconds per request is far
    under the 60 second function timeout and the router's 70 seconds. It
    is over the client's 10 second timeout, accepted: during a long stall
    a user request may time out at the client and finish on the server
    (every write is idempotent); a 5 second budget would fail it instead;
  - rule 9's work budget in Lambda form drops from 20 to 10 seconds per
    request or job call (stop starting new writes after 10 seconds and
    answer `more: true`), so one 11 second stall on the last write still
    fits the 15 second store budget. Change it where it is set: the
    `OutboxDesk` and relay limits (`kit/channels`, `kit/mailin`,
    `apps/notify` `OutboxRelayService`, and any other job's own limit) and
    rule 9 itself; the sign-in path's 3 seconds stays;
  - the store error handlers log the TACHYON storage code with `YonLog`
    (K8's logging step);
  - nothing else: the slices that wait for T1, T13 and other TACHYON
    features use them in their own changes.
- **Depends on:** K8 merged; the TACHYON release approved by the owner.
- **Done when:** gates pass on the new pin; the K2 sign-in check passes 10
  runs in a row on Floci; a 1-hour Floci run under K3's sign-in load shows
  no "store is not available" answer, or the owner accepts a written
  limit (moved here from K8).

### K6. Schedulers call again while `more: true` (a small session, after K3)
Found by K3 (2026-11-12): at 10 writes per call (rule 9, sign-in path) and
one call per minute, removing a 200-member domain takes about 40 minutes,
and `directory`'s removal job works through removing domains in order, so
a small domain waits behind a big one (D2's test timed out at 150
seconds). Decided in EVENTS.md section 4, item 5; no owner question.
- **Owns, for this change only:** `deploy/lambda/scheduler.sh` (OP2 owns
  `deploy/**`: coordinate if OP2 is running), the order of `directory`'s
  removal job (with the directory owner), and the CONTRACTS row of `POST
  /directory/jobs/removals`.
- **Builds:**
  - the Floci scheduler follows EVENTS.md section 4 item 5: it calls a
    route again at once while the answer is `200` with `more: true` (read
    from the body), one call at a time per route, routes in turn, and
    stops at the deadline (`MESAIL_SCHEDULER_SECONDS` minus 10), on `more:
    false`, on another answer or on a timeout; it logs each call as now,
    and the number of calls per route in each tick;
  - `directory`'s removal job takes removing domains in turn: each call
    works on the removing domain after the one the last call worked on (a
    cursor in the job's state), so a big domain does not hold back a small
    one; the person-deletion job too, if it has the same order;
  - CONTRACTS: the removal row says 10 writes and 3 seconds (rule 9, K3),
    and that the trigger calls again while `more: true`.
- **Depends on:** K3 merged (its queue order is what makes the extra calls
  safe).
- **Done when:** on Floci, a 200-member domain is removed within 5 minutes
  while the K2 sign-in check runs with every sign-in under 5 seconds; a
  small domain removed while a big one is still removing is gone within 2
  minutes; gates pass.

### K7. Drain on write within a time budget (the outbox owner, after K4)
Found by K3 (2026-11-12), and seen before K3 (router-cycle 5.6 seconds,
OP1's `503` after 5.0 seconds, K2's run 9.9 seconds): `POST
/api/v1/channels` took 16.6 seconds inside `channels`, because its drain
on write pushed to `messaging` (3.4 seconds), `realtime` (`503` after 5
seconds), `search` (2.2 seconds) and `realtime` again (3.1 seconds) one
after another, and clients timed out at 10 seconds. Every producer has
this path. This is the cause of K3's "also watch" item. Decided in
EVENTS.md section 4, item 6; no owner question.
- **Owns:** `kit/outbox/**` (`RelayDocument`) and its app copies through
  `scripts/kit-sync.sh`; in `apps/channels` only the test that proves it;
  with `realtime`'s owner, the cause of `realtime`'s `503` after 5 seconds.
- **Builds:**
  - drain on write as EVENTS.md section 4 item 6 says: different
    subscribers at the same time, one subscriber in order, 2 seconds per
    push, 3 seconds for the whole drain, unfinished pushes left pending for
    the relay, one outbox write for the results; the relay route keeps its
    rule 9 budget;
  - find why `realtime` answers `503` after 5 seconds on Floci (an upstream
    container behind the router), and fix it or write the cause in the
    hand-over;
  - `channels` cannot write its own log on Floci (a TACHYON handler-log
    gap); if that blocks the work, report it to the coordinator as a
    TACHYON request, not a workaround.
- **Depends on:** K4 merged (both change `kit/outbox`).
- **Done when:** a kit test with a subscriber that waits 10 seconds shows
  `RelayDocument` returning within 3.5 seconds with that push still
  pending, and the relay delivering it later; on Floci, 50 channel
  creations in a row each answer in under 5 seconds; gates pass for every
  app that syncs the kit.

### L7b. Admin app: removals in progress (a new small web-admin session)
L5 stays a mechanical switch with a merge pause, so this UI change is not
folded into it.
- **Owns:** `apps/web-admin/**` (L7's session is finished).
- **Builds:** `DELETE /directory/domains/{domainId}` and `DELETE
  /directory/people/{personId}` answer `202` since D2 (CONTRACTS
  "directory"); the admin app treats `202` as accepted, not as an error,
  and shows the domain as "Removing" (and the person as "Deleting") with
  their actions turned off, until the domain or person list no longer
  shows it (or shows `state: deleted`). Uses the existing admin list and
  badge styles; no new screen.
- **Depends on:** D2 merged.
- **Done when:** gates pass for `web-admin`, the responsive test passes,
  and removing a domain and deleting a person on the native stack show the
  in-progress state and then the end state.

### K2. Sign-in sometimes answers 503 on Floci (identity's successor, with `mesail-keycloak`)
Seen 2026-11-12 in L5, OP1 and router-cycle runs: `POST
/api/v1/identity/session` waits 5 seconds inside `identity` and answers
`503` "sign-in is not available now"; `TestD2DeletingAPersonAnswers202...`
failed 2 of 3 times. No `directory` or mail call was slow in the router's
log for that window.
- **What is known:** every outbound call of `identity` (Keycloak token,
  Keycloak admin, `directory`'s `GET /people/{personId}`) uses one 5-second
  timeout (`kit/oidc` `VerifierFactory.fetchTimeout`) with
  `kit/transport` `SingleUseTransport`, so the keep-alive bug is ruled
  out. Keycloak calls do not pass through the router, so its log cannot
  show them.
- **First suspect:** Keycloak on SQLite has one writer. The failing test
  deletes a Keycloak user (through `directory`) while another test signs
  in; a token grant that waits on SQLite's write lock for more than 5
  seconds gives exactly this answer. Also check a cold `identity`
  function, and a `GET /people` queued behind `directory`'s single writer
  during D2's removal job (each job call may run up to 20 seconds, SLICES
  rule 9).
- **Builds:**
  - `identity` logs which upstream call timed out (Keycloak token,
    Keycloak admin or `directory`) and how long it took, with no secret or
    person data;
  - a repeatable check on Floci: delete a person while signing in a loop;
  - the fix that the check points to, for example SQLite settings in the
    Keycloak image (WAL journal, a busy timeout), separate timeouts for
    Keycloak and for `directory`, or both. If the cause is `GET /people`
    waiting behind `directory`'s writer, report it to the architect before
    fixing: that is a design question (sign-in depends on a single-writer
    function).
- **Done when:** the D2 deletion test and the sign-in tests pass 10 runs in
  a row on Floci, and the cause is written in the slice's hand-over.

### K1. Clean-up after the token switch (one small session)
Open items from L5 (`~/Library/Logs/local/CADUCEUS/mesail/l5-switch-note.md`).
None needs the owner. The messaging and mail-in call cycle has its own fix
session; S3 blobs for `mail-out` is a later cloud-form item (ADR 0007
section 7).
- **Owns, for these changes only:** `apps/directory` `TenantService` (with
  the directory owner), the dev seed in `deploy/floci/**` and
  `scripts/native/**` (with P1), `edge/smtp` service-token code (with M1's
  successor), `apps/directory` D2 test setup, the web-admin test
  `DomainListServiceTest.go`, `tests/responsive/IdentityFake.go`, and
  `tests/blackbox/README.md`.
- **Builds:**
  - **"Hosted" means `active`** (CONTRACTS "directory", decided
    2026-11-11): `TenantService.Hosted` counts only `active` domains, as
    `GET /addresses` already does, so recipient kinds and `mail-out`'s
    hand-off agree with `mail-in`. The fixture follows L0: the seed adds
    `partner.test` and leaves it `pending` (never verified), and `carol` is
    a member of it. So `carol@partner.test` resolves `external`, and carol
    can sign in (a `403 not_a_member` for `carol@example.com`).
  - **`edge/smtp` gets service tokens from `identity`**: it uses `kit/oidc`
    `ServiceTokens` (JSON to `POST /api/v1/identity/service-token`, domain
    `*` for its install-wide calls) instead of OAuth client credentials with
    a form body; its `MESAIL_SERVICE_TOKEN_URL` setting is replaced by the
    `servicetokens` URL. Needed before OP2 deploys the SMTP edge.
  - **D2's 200-member test gets its own empty FYLO root** for each run (or a
    fresh domain key), so its time no longer depends on what other tests
    left behind; its deadline stays.
  - **web-admin's `DomainListServiceTest`** uses the install-wide domain `*`
    (`oidc.InstallWide`), not `mesail-operators`.
  - **`tests/responsive/IdentityFake.go`** accepts `login`, not `address`.
  - **Document size check** (DOMAIN.md "Every document stays under 512
    KiB"): for each FYLO-owning service, find its largest possible
    document at today's caps and add a real-FYLO test at that size; at
    least `messaging`'s email `Message` (body, `blocksJson` and a staged
    `message.created` that carries the body), `search`'s index document and
    `notify`'s summary. Lower a cap where a document can pass 512 KiB, and
    tell the architect which caps changed.
  - **`mail-in`'s `inbound-content` is split by month** (DOMAIN.md "FYLO
    layout", decided 2026-11-12): `kit/mailin` `ContentShelf` uses
    `CollectionName{}.ForDomainMonth` with the email's received month, and
    reads look in that month's collection (the content's key carries the
    month, or the reader finds it from `InboundRaw`'s received time). Each
    email's content may be up to about 2 MiB, so one collection per domain
    would reach FYLO's 256 MiB limit after about 128 large emails. With the
    mail-in owner's agreement; no data migration is needed before the first
    release.
- **Done when:** gates pass for every touched app and module, and the
  blackbox recipient and sign-in tests pass natively and on Floci.

### D2. Directory: long operations in batches, and repairs (L3's successor)
Open items from L3 (develop 3d984bc, hand-over
`~/Library/Logs/local/CADUCEUS/mesail/l3-directory-note.md`). None needs
the owner.
- **Owns:** `apps/directory/**` (except kit copies).
- **Builds:**
  - **Batches (SLICES rule 9: at most 25 writes per call).** Removing a
    domain and deleting a person first write one state change, then finish
    in a scheduled job (rule 13) in batches of at most 25 writes, each step
    idempotent. `DELETE /domains/{domainId}` sets `state: removing` and
    answers `202` with the `Domain`; a removing domain is left out of every
    `PersonRecord.domainIds` at once, so no sign-in, refresh or switch
    reaches it, and mail for it is refused. `DELETE /people/{personId}`
    deletes the Keycloak user and sets `state: deleted` first (that ends
    every sign-in), answers `202`, and removes the memberships in batches.
    CONTRACTS changes both rows from `204` to `202` in the same commit.
  - **Release alias reservations of a removed domain.** The removal job
    drops the domain from each `local-parts` document with `owner:
    addresses`, and deletes a document whose `domainIds` becomes empty. A
    person's prefix is never released (SPEC Q5).
  - **Recover a Keycloak user left by a failed add.** `directory` makes
    each Keycloak user with the attribute `mesail.addRequest` = an id it
    also writes into the prefix's `local-parts` reservation before the
    call. When a later add of the same prefix gets Keycloak's "user
    exists" and no person has that username, `directory` reads that user:
    if its `mesail.addRequest` matches the reservation, `directory` made
    it, so it finishes the add with that user's `sub`; otherwise it answers
    `409 duplicate` as now (CONTRACTS "directory", no take-over of a user it
    did not make).
- **Not here:** `people.sessionsEndedAt` and its route are slice O2 (Wave
  4); L3 left them out on purpose.
- **Depends on:** L3 merged. The batch job follows the scheduled-work rule
  (EVENTS.md section 4).
- **Done when:** a test removes a domain with 200 members in batches of at
  most 25 writes with no member left; a test deletes a person in 5
  domains; a test makes Keycloak fail after the user is made and shows the
  next add of the same prefix finishes; gates pass.

## Wave 3b: browser push (SPEC AC-PN-3, decided 2026-11-11)

The owner chose to add browser push to the first release. The server side
exists (slice N1: subscriptions, `GET /notify/push-key`, VAPID and RFC 8291
sending); this wave adds the payload format and the browser side.

### N2. Browser push (N1's successor for `notify`, F4's successor for the web app)
- **Owns:** in `apps/notify/**`, the push payload code; in
  `apps/web/client/**`, F4's `NotificationPush*` files, the push settings
  row and the one-time suggestion, and the `push` grant in the web app's
  `tac.browserApis` (coordinate the manifest line with F1's owner).
- **Builds:**
  - `notify`: the payload JSON of CONTRACTS "notify" (title, body without
    message text, path, tag), with tests that no message text can reach
    it; deleting the member's push subscriptions in a domain on
    `member.upserted` with `state: removed` (EVENTS.md section 7).
  - web: `Push.Supported`; the settings toggle and the one-time
    suggestion; on the click, `GET /notify/push-key`, `Push.Subscribe`,
    then `POST /notify/push-subscriptions` with `PushSubscribed`'s fields;
    on opening another domain while push is on, the same registration in
    that domain; `Push.Unsubscribe` on sign-out and on every device-copy
    wipe; the path route that opens a conversation in its domain (switching
    the tab's domain with `POST /identity/session/domain` when needed);
    the denied state ("Blocked in this browser's settings").
  - The push row and the suggestion stay hidden when `GET
    /notify/push-key` answers `503` (push turned off with
    `notify.push.enabled`, or not configured; OP1).
  - The screens are merged (develop a360499; handover
    `~/Library/Logs/local/CADUCEUS/mesail/n2-push-ux-handover.md`): the
    toggle, the suggestion (hidden for good after "Not now" or closing),
    the prompt moment, the denied state, the iPhone and iPad Home Screen
    hint, and a click that opens the first unread message.
- **Depends on:** Mesail on a TACHYON release that has the push client
  (ADR 0118; not 26.46.02); Wave 3 (one login) merged for the domain
  switch.
- **Done when:** in Chromium and Firefox on the dev stack, with no Mesail
  tab open, a mention shows a system notification without message text,
  and a click opens the conversation in the right domain; signing out
  stops further pushes.

## Wave 4: offline first (ADR 0006)

Starts after Wave 3 (one login) is merged; LU-style design work (OU) and
the blackbox tests (O0) may start earlier. SPEC S15 to S17 and AC-OF-1 to
AC-OF-17; CONTRACTS "messaging" (Sync), "identity" (kept devices,
`DELETE /sessions`, ended elsewhere), "directory" (`sessionsEndedAt`).

### What changes where

| Area | Change |
| --- | --- |
| `apps/messaging`, `kit/messaging` | The `changes` log (written by the relay, kept 30 days, a monthly cleanup job), `POST /sync`, `lastChangeId` and `readMessageId` in the conversation list, `Message.version` and `writtenAt`, `ifVersion` and `409 edit_conflict`, `409 message_deleted`, idempotent delete and reactions, forward-only read positions. |
| `apps/identity` | `keepSignedIn` (Keycloak `offline_access`), `Session.kept`, `signedInAt` in the `sessionKey`, `403 access_revoked` for ended-elsewhere sessions, `DELETE /sessions`, `sessions-ended` call on the person's own password change. |
| `apps/directory` | `people.sessionsEndedAt`, `POST /people/{personId}/sessions-ended`, set on the AC-ID-13 account actions; `PersonRecord.sessionsEndedAt`. |
| `deploy/keycloak` | Client scope `offline_access` for `mesail-identity`; offline session idle 30 days and at most 90 days; revoking offline sessions through the admin API. |
| `apps/web/client` | The `DeviceStore` port and its TACHYON `Store` adapter (partitions per person and domain, page rows, budget, wipe), the sync engine, the queue, "Needs your attention", offline recipient chips, offline search, the kept-device sign-in and storage of the `sessionKey`, the offline states of every screen. |
| `realtime`, `channels`, `search`, `attachments`, `notify`, `web-admin` | No change. Realtime events become pokes on the client side. |
| `docs/mesail/ux/**` | The offline states (OU). |
| `tests/blackbox` | O0. |

### OU. UX for offline (`mesail-ux`)
- **Owns:** `docs/mesail/ux/**`.
- **Builds:** "Keep me signed in on this device" at sign-in; the offline
  status ("Offline, updated 08:12"); "Waiting to send" and "Written
  offline at" on messages; the "Needs your attention" list with Edit,
  Retry, Send by email and Discard; "Not on this device"; "On this
  device" search results; the edit-conflict screen (newer text, the
  offline text as a draft); the one-time "not sent" notice when access is
  lost; the sign-out warning with queued items; the space settings (used
  per domain, window, "Remove the copy on this device"); every width.
- **Depends on:** nothing.

### O0. Blackbox tests for sync (architecture, `mesail-arch`)
- **Owns:** `tests/blackbox/**`.
- **Builds:** contract tests for the sync route (`changed`, `unchanged`,
  `reset`, `gone`, paging, the 5-second overlap), `lastChangeId`,
  `ifVersion` and `edit_conflict`, `message_deleted`, idempotent delete and
  reactions, forward-only read positions, `writtenAt`, `keepSignedIn`,
  `DELETE /identity/sessions` and `403 access_revoked`.
- **Depends on:** nothing; the tests fail until O1 and O2 land.
- **Landed:** `Sync_test.go`, `Queue_test.go`, `KeptDevice_test.go`
  (helpers in `Sync.go`).

### O1. Messaging: change log and sync (M3's successor)
- **Owns:** `apps/messaging/**` (except kit copies of other slices),
  `kit/messaging/**`.
- **Builds:** CONTRACTS "messaging" sync rules and the changed routes;
  DOMAIN.md "Change"; the relay step that appends change rows before
  pushing, idempotent by `eventId` (rule 9: one write per row, at most 25
  per call); the monthly cleanup as a scheduled job (rule 13).
- **Depends on:** Wave 3 merged (Mesail tokens).
- **Blackbox:** O0's sync and conflict tests.
- **Done when:** the app bundles, starts and answers `GET /health` 200,
  and a client that was offline for 1,000 changes catches up within 15
  seconds on the dev stack.

### O2. Kept devices and ended sessions (identity's and directory's successors, with `mesail-keycloak`)
- **Owns:** `apps/identity/**`, the `sessionsEndedAt` part of
  `apps/directory/**` (coordinate with the directory owner),
  `deploy/keycloak/**` for the offline session settings (`mesail-keycloak`
  does that part).
- **Builds:** CONTRACTS "identity" `keepSignedIn`, `kept`, `signedInAt`,
  `DELETE /sessions`, ended elsewhere; CONTRACTS "directory"
  `sessionsEndedAt` and its route. Also the `sessions-ended` call on the
  removal of one passkey (PK2 ends the other Keycloak sessions until
  then, so the other sessions get `401`, not `403 access_revoked`), and in
  `directory`'s removal of all passkeys (PK3 ends every Keycloak session
  of the person until then); PK0's
  `TestAcId17RemovingAPasskeyEndsTheOtherSessions`,
  `TestAcId16PasskeySignInMayKeepTheDevice` and
  `TestAcId13AdminRemovesAllPasskeysAndEndsTheSessions` pass only after O2.
- **Depends on:** Wave 3 merged.
- **Blackbox:** O0's session tests.
- **Done when:** a kept session refreshes after a restart of `identity`, and
  "Sign out on all devices" makes the other kept session answer `403
  access_revoked`.

### O3. Web: the device store (F1's successor)
- **Owns:** `apps/web/client/**` files named `DeviceStore*`,
  `DevicePartition*`, `DeviceBudget*`, `DeviceWipe*` and the `Device*`
  entities and models; the kept-device parts of `Session*`.
- **Builds:** the `DeviceStore` port and its adapter on TACHYON's `Store`
  with the IndexedDB adapter (ADR 0006 sections 1, 6 and 9): with TACHYON
  T1, one row per message partitioned by
  `<personId>/<domainId>/<conversationId>`, eager tables for the
  conversation list, cursors, queue and drafts, load and unload of
  partitions, `StoreWriteFailed` handling; attachments through `Blobs`
  (T2) with AC-OF-2's full window; every table and the blob store sealed
  with `"sealed": "whenAvailable"` (TACHYON T6b, in wave 3 with sealed
  tables), the warning of SPEC AC-OF-18 on the fallback event, and the
  plain-to-sealed refill when a device can seal later; `StorageSpace.persist()` and
  `estimate()` (T9), the 30-day lock, storing the kept `sessionKey`, and
  wiping on sign-out, another person, `access_revoked` and a lost domain
  (`StorageSeal.forget()` first). Behind the release gate below.
- **Depends on:** OU; Wave 3's L6 merged.
- **Done when:** gates pass, the copy survives a restart of the app in a
  browser and in the macOS and Android shells, and every wipe case leaves
  no row of the person.

### O4. Web: sync engine and queue (F2's and F3's successors)
- **Owns:** `apps/web/client/**` files named `Sync*`, `Queue*`,
  `Attention*`, and the offline parts of `Conversation*` and `Composer*`.
- **Builds:** catch-up and pokes (ADR 0006 section 3; also on T4a's
  `PushReceived`), the queue with ordering per conversation, backoff and
  "Needs your attention" (section 4), sent by the tab that holds the T11
  `Locks` lock, the T5 reconnect policy that never gives up, conflict
  handling (AC-OF-8), offline recipient chips (AC-OF-5), access-lost
  handling (AC-OF-9), the sync bar as a status that never takes focus,
  with a "Details" button in its actions slot that opens the sync panel
  (not the whole bar as one button, as the mockups draw it; matches DuVay
  `w-sync-status`), files attached offline (kept with `Blobs`, uploaded
  before their send, AC-OF-4), the in-memory queue of a tab that is not
  kept (AC-OF-19), the offline states from OU. The sync engine is a
  service with no view binding, so it can move to a T10 worker later.
- **Depends on:** O1, O2 and O3.
- **Done when:** S15 and S16 work on the dev stack with the network turned
  off and on, and two tabs never send an item twice.

### O5. Web: offline search (F4's successor)
- **Owns:** `apps/web/client/**` files named `OfflineSearch*`, and the
  offline parts of `Search*`.
- **Builds:** the in-memory word index of the device copy with AC-SR's
  query rules, "On this device" results, the switch between device and
  server search on `ConnectivityChanged`.
- **Depends on:** O3.
- **Done when:** a search offline finds every message of the copy that the
  same query finds on the server.

### Order

1. At once, after the coordinator's go: **OU** and **O0** (they may run
   during Wave 3).
2. After Wave 3 is merged, in parallel: **O1**, **O2**, **O3**.
3. After O3: **O5**. After O1, O2 and O3: **O4**.
4. **DuVay D1** (offline-state components, requested in ADR 0006): on
   DuVay `develop`, web only, styled per OS skin, not yet in a DuVay
   release (spec and helpers 5991376, `docs/offline-states.md`;
   `w-storage-meter` 0c6aa9a and ca38bee; `w-send-status` and the "Needs
   your attention" list pattern 64be0b7; `w-sync-status` d68028d). Until
   a DuVay release has them, O3 and O4 build Mesail's own parts with
   Tailwind, as `docs/mesail/ux/COMPONENTS.md` "Offline and sync" says.
   At the DuVay pin move to that release (the slice that moves the pin,
   rule 14), they replace, one for one: the sync bar (`.ms-sync-bar`,
   `SyncStatusService`) by `w-sync-status`; the message queue notes
   ("Waiting to send") by `w-send-status`; the attention page cards
   (`.ms-attention-card`) by DuVay's "Needs your attention" list pattern;
   the space meter (`.ms-space-meter`) by `w-storage-meter`. If that DuVay
   release comes before O3 or O4 build these parts, they use D1 at once.
5. **Release gate** (SPEC Q16, decided "Wait for encryption"; ADR 0006
   section 9): O1 and O2 are server changes and ship as soon as they
   merge. O3 needs TACHYON's T1, T9 and T2 (waves 1 and 2) to build, and
   sealed tables (wave 3) to finish. O3, O4 and O5 may merge earlier, but
   "Keep me signed in on this device" stays hidden and no device keeps a
   copy until sealed tables are in a TACHYON release that Mesail uses and
   O3's tables are declared sealed. Turning it on is one small last slice
   (show the box, run S15 to S17 on the dev stack in a browser and the
   macOS, Windows, Linux and Android shells).

## Wave 5: on-premise (ADR 0007)

The owner's requirement of 2026-11-11: no dependency on any cloud
resource; Mesail runs on-premise on its own. SPEC AC-OP-4 to AC-OP-8 and
Q17 to Q20. Most of the work is already done: the code is cloud-neutral
except for the items below.

### OP1. Remove public and cloud defaults (a small cross-app session)
- **Owns, for this change only:** `apps/directory/server/application.json`
  and `apps/directory/server/services/DnsService.go` (with the directory
  owner), `kit/blobs/S3Settings.go` and its app copies, `edge/smtp` raw
  store settings, `apps/notify/server/application.json` and
  `kit/webpush` (with the notify owner), the realtime gateway stage of the
  `Dockerfile` and `edge/realtime` vendoring (with P1 and R1).
- **Builds:** `dnsresolver` defaults to the host's own resolver, with no
  public host in `outgoing.hosts` (the dev and Floci runs keep their own
  resolver setting); the S3 adapter (blobs and the SMTP edge raw store)
  refuses to start without an explicit endpoint, with no
  `amazonaws.com` fallback; a `notify.push.enabled` setting (default
  `false` for on-premise installs; SPEC Q19) that hides the web app's push
  row through `GET /notify/push-key` answering `503`; `github.com/coder/
  websocket` vendored, so every Go build runs with `GOPROXY=off`.
- **Depends on:** nothing. May run during Wave 3 and 4.
- **Also (decided 2026-11-11, after L5 merges):** rule 17 for `mail-out`:
  `mailout.directenabled` and `mailout.relayrequiretls` accept `"true"`
  and `"false"` from the environment (this change only, with the mail-out
  owner's agreement). The Floci blob layout for `attachments` moved to OP2
  (decided 2026-11-12 with OP1): Floci names each EFS volume only after
  deploy, while the router starts before deploy, so mounting it needs a
  restart workaround. The Floci blackbox suite reaches `attachments` only
  through its health route, so it stays green.
- **Done when:** a grep of the tree finds no public host or AWS host as a
  default in code or settings (`cloudflare-dns`, `amazonaws.com`,
  `8.8.8.8`, `1.1.1.1`, `dns.google`), and gates pass for the touched
  apps and modules.
- **Built** (branch `feat/op1-defaults`): `directory` uses
  `dnscheck.SystemLookup` (Go's resolver, so `/etc/resolv.conf`) when
  `dnsresolver` is empty, and the DoH lookup only for a set URL;
  `blobs.NewS3Store` returns `MissingEndpoint` without an endpoint
  (attachments `503`, mail-in `MissingBlobs`, the SMTP edge requires
  `MESAIL_RAW_STORE_S3_ENDPOINT`); `notify.push.enabled` (default
  `false`) makes `GET /push-key` answer `503` and marks waiting pushes
  `skipped`; `edge/realtime/vendor/` with `GOPROXY=off -mod=vendor` in
  the Dockerfile. The module `tests/nocloud` (run by `gates.sh
  --shared-only`) fails on any public or cloud host in code or settings
  and checks both defaults. The web app's push row is N2's: it does not
  exist yet, and N2 hides it on that `503`. Left for OP2: on Floci, real
  `attachments` calls answer `503` (no S3 endpoint, no blob adapter set)
  until OP2's blob layout; `mail-in` and `mail-out` already use L5's
  shared folder. The push vendor hosts stay in notify's `outgoing.hosts`
  and `push.hosts` (used only when push is on).
  Rule 17: `notify.push.enabled` and mail-out's `directenabled`,
  `relayrequiretls` and `relayimplicittls` (`kit/mailout` `Switch`) take
  `true`/`false` as JSON or text; other text makes every mail-out call
  fail with `InvalidSwitch`. mail-out's number settings (`directport`,
  the delays) still read JSON numbers only.

### OP2. The on-premise runtime: Lambda in Floci on kestrel (P1's successor)
- **Owns:** `deploy/**` (except `deploy/keycloak`), `docker-compose*.yml`,
  `scripts/floci/**`, `scripts/native/**`, `scripts/onprem/**` (new), the
  non-Keycloak `Dockerfile` stages.
- **Builds** (SPEC Q18, decided "Lambda in Floci"; ADR 0007 section 4):
  - a production definition for a dedicated VM on kestrel with its own
    Docker engine, deployed through PaaS like FitNest: Floci and the Lambda
    functions (one per service and domain, ADR 0004 section 4), the router
    in Lambda mode, the scheduler, `realtime`, `edge/realtime`,
    `edge/smtp` (it has no deployment yet), Keycloak with its data on a
    volume (not tmpfs) and the operator's settings (OPERATIONS.md);
  - the shared blob folder (ADR 0007 section 7), in the Floci test run and
    in production: `attachments` joins L5's EFS share (`fileSystemShare:
    "mail"`, folder `/mnt/data/blobs`) on the local adapter, and the router
    and the SMTP edge reach the same folder for `/blobs` and
    `MESAIL_RAW_STORE_DIR`. Floci names each EFS volume only after deploy
    (`floci-aws-efs-<access point>-<hash>`), and the router starts before
    deploy, so do not rely on that generated name: find a layout where the
    folder exists first (for example Floci's EFS on a fixed host path, or
    a named volume made before deploy) or, if Floci allows nothing else,
    start the router after deploy; no restart of a running router. Add a
    Floci blackbox run that uploads and downloads an attachment, since the
    suite reaches `attachments` today only through its health route;
  - networks: an `internal` network for Floci, the functions, Keycloak and
    `realtime`; one egress container as the only way out, with an
    allowlist built from the settings (resolver, relay or port 25 for
    `mail-out`, push hosts for `notify` when on), which logs every refused
    attempt; `edge/smtp` gets inbound port 25 and the resolver only;
  - Floci's port published on no interface; the deploy tooling runs
    inside the VM;
  - a registry inside the install for Floci and every Mesail image; the
    functions' image addresses point to it; nothing pulls from a public
    registry at start or cold start;
  - blobs (ADR 0007 section 7): one folder on the VM's data disk, as one
    Floci EFS access point `blobs` mounted by `attachments`, `mail-in` and
    `mail-out`, and as a plain volume for the router (`/blobs`) and the
    SMTP edge (`MESAIL_RAW_STORE_DIR`); all use the local adapter. L5's
    shared EFS access point for the Floci test run is the start of it;
  - DKIM keys for real domains: `scripts/onprem/dkim-key.sh <domain>`
    makes a key pair for a hosted domain, writes the private key as a
    secret file that `mail-out` reads (`MESAIL_SECRET_*`), and prints the
    DNS record to publish (L5 only adds test keys);
  - warm functions in production: the scheduler calls every function's
    health route every 5 minutes, and once after each deploy, so a user
    request rarely meets a cold start (Floci's cold start is about 10 s;
    K2 saw `identity` open new cold environments under concurrent sign-ins,
    and a service-token call take 5 s). Measure `identity` under 20
    concurrent sign-ins and report whether a per-function minimum of warm
    instances is needed;
  - ready before use (found by K3, 2026-11-12: the first run 60 seconds
    after the VM booted got `503` and `502` from the seeds, `directory`
    hit Floci's 30 second Lambda timeout, and the seeds' 60 second service
    token expired while they were slow): the start script and the test
    runner wait until every function answers its health route and one
    sign-in passes before they seed or test, and a seed gets a new service
    token when its token expires;
  - Floci kept pinned by digest, with an upgrade note;
  - a check of Floci's licence and maintainer, reported to the owner if it
    does not allow production use.
- **Depends on:** Wave 3's L5 merged; OP1.
- **Done when:** the stack starts in the VM on kestrel (or in the `mesail`
  dev host as a stand-in) from this definition and passes OP3.

### OP3. Outbound-blocked gate for the Floci stack (C1's successor, with P1)
- **Owns:** `scripts/onprem/egress-check.sh` (new), its CI step.
- **Builds:** starts OP2's Floci stack with every route out blocked except
  the egress container's allowlist, runs the blackbox tests (external mail
  and push tests skip), and records every refused outbound attempt from
  any container, Floci and the function containers included (from the
  egress container's log and a DNS log); any refused attempt that is not a
  test of the allowlist fails the gate (SPEC AC-OP-8). Also checks that no
  image is pulled from a public registry during the run.
- **Depends on:** OP2.

### OP4. Operator guide (architecture)
- **Owns:** `docs/mesail/OPERATIONS.md` (new).
- **Builds:** what an on-premise operator must provide and set: host
  disks, the resolver, the relay or direct delivery (port 25 out, reverse
  DNS), inbound port 25 and MX records, TLS certificate files and the CA,
  NTP, secret files, Keycloak backups, FYLO `pack` backups, push on or off.
- **Depends on:** OP2's settings names.
- **Landed (first version, 2026-11-11):** `docs/mesail/OPERATIONS.md`, with
  the planned settings marked; OP1 and OP2 update it when they add or
  rename a setting.

Later, only for a cloud form (not on-premise): `mail-out` gets the S3
blob adapter that `mail-in` has (ADR 0007 section 7).

OP5 (an offline build bundle) is dropped: the owner decided that builds
may download pinned parts (SPEC Q20).

### Order
OP1 and OP4 at once; OP2 after L5 and OP1; OP3 after OP2.

## Wave 6: passkeys (ADR 0008)

The owner's request of 2026-11-12: biometrics (Touch ID, Face ID and the
like) as a sign-in option. SPEC S18, AC-ID-15 to AC-ID-19, Q21 and Q22;
CONTRACTS "identity" (passkeys). TACHYON's plan (approved 2026-11-12):
T13 (web passkeys) in its wave 2, already started; T15 (biometric unlock)
in its wave 3b, after the sealed tables; T14 (app passkeys) first in its
wave 4. T15b (a run-time on/off switch for the unlock) was accepted into
TACHYON's T15 slice (ADR 0008). The Keycloak and identity parts may be built before.

### PU. UX for passkeys (`mesail-ux` or a UX session)
- **Owns:** `docs/mesail/ux/**`.
- **Builds:** "Sign in with a passkey" on the sign-in form (and in the
  username field's autofill where the browser offers it), its failure
  lines; Settings > Sign-in: the passkey list (name, added, last used),
  "Add a passkey" with the password and code step, rename, remove (with
  the "other devices are signed out" warning); the admin's "Remove all
  passkeys" in the person menu (AC-ID-13); the app's "Unlock with Touch
  ID" setting and locked screen (AC-OF-20); hidden where the device has no
  passkey support.

### PK0. Blackbox tests for passkeys (architecture, `mesail-arch`)
- **Owns:** `tests/blackbox/**`.
- **Builds:** contract tests for every passkey route, using a software
  WebAuthn authenticator in the test (ES256 key pair, made-up credential
  id, user verification flag set): add, list, sign in, wrong signature,
  replayed challenge, expired challenge, another person's passkey, remove
  and `access_revoked` on the other sessions, the admin removal, 10
  passkeys at most.

### PK1. Keycloak passkey flow (`mesail-keycloak`)
- **Owns:** `deploy/keycloak/**`.
- **Builds:** in the realm `mesail`: the WebAuthn passwordless policy
  (relying party id from a setting, user verification required, resident
  key required, attestation none, ES256 and RS256, at most 10 per user); a
  direct-grant flow for passkeys with a custom authenticator that takes
  the assertion, the expected challenge and the allowed origins from
  `identity`, finds the user by the user handle, and checks them with
  Keycloak's WebAuthn code (brute-force detection counts failures); admin
  endpoints for `mesail-account` (register a passkey for one user, list,
  rename, remove) and for `mesail-directory` (remove all of one user).
- **Done when:** `oidc_check.py` signs in with a software authenticator
  through `mesail-identity`, and refuses a replayed or wrong assertion.

### PK2. Identity passkey routes (identity's successor)
- **Owns:** `apps/identity/**`, `kit/oidc` session and account code for
  passkeys.
- **Builds:** CONTRACTS "identity" passkey routes (rename and
  `setup_expired` included, added 2026-11-12 for PU), the sealed challenge
  and `setupKey`, `identity.passkeyorigins`, `sessionsEndedAt` on removal.
- **Depends on:** PK1.

### PK3. Directory: remove all passkeys (directory's successor)
- **Builds:** `DELETE /directory/people/{personId}/passkeys` with the
  AC-ID-13 check, through PK1's admin endpoint.
- **Depends on:** PK1.

### PK4. Web: passkey sign-in and settings (the web app's owner)
- **Builds:** PU's screens with TACHYON's `Passkeys` client (T13, TACHYON
  ADR 0126): `passkeys` in the web app's `tac.browserApis` (coordinate the
  manifest line with F1's owner); `check()` and its `PasskeySupport` event
  decide whether "Sign in with a passkey" and Settings > Sign-in show
  passkeys; `create` and `get` are called directly from the button's click
  handler, never after an await or a timer; `offer` for the username
  field's autofill; every `PasskeyFailed` reason has a line (PU).
- **Depends on:** PU, PK2, TACHYON T13 in a release Mesail uses.

### PK5. Apps: passkeys in the native shells (P1's successor, with the web app's owner)
- **Builds:** the manifest settings for T14 (relying party id, Apple
  associated domain, Android signing certificate), the router's
  `/.well-known/apple-app-site-association` and
  `/.well-known/assetlinks.json` from the install's settings, and the
  native origins in `identity.passkeyorigins`; OPERATIONS.md: each install
  builds its apps with its own host.
- **Depends on:** PK4, TACHYON T14.

### PK6. Biometric unlock of the device copy (SPEC Q22 decided yes; AC-OF-20)
- **Builds:** sealed tables with `"unlock": "userPresence"` (TACHYON T15)
  for the device copy in the installed apps, the "Unlock with Touch ID"
  setting, the locked screen (on open and after 5 minutes in the
  background) and its PU screens. For AC-OF-20's "how many unsent items
  were lost": the app keeps the number of waiting queue items (a count
  only, no content) outside the OS-locked tables, under the normal sealed
  key, and updates it with every queue change, because after `keyLost` the
  locked queue cannot be read. The locked screen shows no counts.
- **Depends on:** Wave 4's O3, TACHYON T15.

### Order
PU, PK0 and PK1 at once; PK2 and PK3 after PK1; PK4 after PK2 and T13
(TACHYON wave 2); PK5 after PK4 and T14 (TACHYON wave 4); PK6 after Wave
4's O3 and TACHYON T15 (wave 3b) with T15b. PK1 and PK2 accept the app
origins of ADR 0008 T14 (`https://<host>` for Apple and Windows,
`android:apk-key-hash:<hash>` for Android). PK4 uses the grant `passkeys`
and handles every `PasskeyFailed` reason. PK6 declares `"unlock":
"userPresenceWhenAvailable"` and uses `StorageLock.require` (T15b).

## Wave 7: thin controllers and numbered errors (ADR 0009)

The owner's rule of 2026-11-12 (SLICES rule 18). Where the Services are
today: `directory` (37 files), `notify` (40), `messaging` (10) and
`mail-in` (1) have `server/services/`; `attachments`, `channels`,
`identity`, `mail-out`, `realtime` and `search` have none, and their
controllers call `server/packages/<domain>/` directly.

Every E slice below:
- **Owns:** its apps' `server/controllers/**`, `server/services/**`,
  `server/errors/**` and `server/models/ApiError.go`, the app's tests,
  and its rows in `docs/mesail/ERRORS.md`.
- **Builds:** each route method as one `return` of one Service call; a
  Service for every controller that has none (as TACHYON providers, the
  pattern `directory` uses); the caller lookup, checks and wire mapping
  moved into the Services; each error type carries its number; the error
  handler formats `[MSnnnn]: <message>` with `kit/apierrors`, keeps every
  `code` and `field` as CONTRACTS says, and answers the app's
  "unexpected error" number for an error no other method takes; its rows
  in ERRORS.md. No change in routes, statuses, `code` values or behavior.
- **Done when:** the E0 gate check passes for the app; the app's tests
  check the number on each error answer; the blackbox tests that touch
  the app still pass where they passed before; gates pass.

### E0. The shared parts (a small session)
- **Owns:** `kit/apierrors/**` (new), `tools/controllers/**` (new), the
  line in `scripts/gates.sh` that runs it (with C1's owner).
- **Builds:** `kit/apierrors`: `Message(number int, text string) string`
  (`[MS0510]: text`, four digits), a `Numbered` interface for error types
  (`Number() int`), and a test helper that reads `docs/mesail/ERRORS.md`
  and checks that a list of numbers is inside a block and listed once;
  `tools/controllers`: a Go check that every exported method of a
  `server/controllers/*Controller.go` type has one statement, a `return`
  of a call on one of the controller's own fields; it runs in
  `scripts/gates.sh` for each app, with a list of apps not converted yet
  that Wave 7 empties.
- **Done when:** both have tests; gates pass with every app on the list.

### E1. `attachments` and `search`
### E2. `mail-in` and `mail-out`
### E3. `channels` (not at the same time as K7, which adds a test there)
### E4. `realtime` (after K5)
### E5. `notify` (after K5)
### E6. `identity` (after PK2 and K1; before O2 if O2 has not started)
### E7. `directory` (after PK3 and K8; not at the same time as K6)
### E8. `messaging` (after K5, K8 and K9; K4's test stays)
### E9. The router's answers (`deploy/router`; after K8, not at the same time as OP2)
- **Builds:** the router's own answers (`429`, `502`, `503`, `504`) as
  `ApiError` with numbers from MS5500-MS5999.

### Requests to TACHYON (ADR 0009)
- **Y1:** let `@ErrorHandler` shape the answers TACHYON makes itself (no
  identity, missing role, identity provider failure, `@Check` rejection,
  malformed or too large body, an error no other method takes), keeping
  TACHYON's status codes. When it is released and the pin moves, each
  app's handler adds these numbers.
- **Y2 (not blocking):** pass the identity that `@IdentityProvider`
  returned into a route method parameter. When it lands, a small
  follow-up replaces the `request` argument of every route with it.

### Order
E0 first. Then at once: E1, E2, E3. Then each other E slice when the
slices it names are merged. K10 (store error logging) changes the same
error handlers: whichever of K10 and an app's E slice merges second
rebases onto the first.

## Later (after the first release)

- **X1** SMTP submission for third-party mail clients (app passwords or
  SASL OAUTHBEARER).
- **X2** Port external accounts, calendar and contacts sync from
  `internal/` at commit 4ef82f72 into a Yon app.
- **X3** CADUCEUS data migration tool.
- **X4** Interface translations and in-browser message translation.
- **X5** Split `web` into separately deployed feature bundles once TACHYON
  can mount separately built Tac apps.
- **X6** Use FYLO's word index and sorted pages (FYLO ADR 0023, on FYLO
  `develop` since 2026-11-11, not yet in a FYLO release Mesail pins) in
  `search` and in `messaging`'s newest-first lists, replacing Mesail's own
  word index (`kit/textindex`) where it is simpler. FYLO sorts by number
  only, so add numeric companion fields (for example `createdAtMs`, epoch
  milliseconds) next to the RFC 3339 times; the wire format stays RFC 3339.
  Check the index size first: FYLO estimates 6 to 12 KB of word-index keys
  per 2 KB message body against its 256 MiB limit per collection, so a
  busy domain's monthly `search-index` collection may need a finer split
  (DOMAIN.md "FYLO layout").
- **X7** Consider FYLO's change feed (FYLO ADR 0024, on FYLO `develop`
  since 2026-11-12) instead of `messaging`'s own `changes` log (ADR 0006
  section 3). Before switching, check two limits: it needs FYLO's segments
  layout, and a cursor expires after a pack, a rekey, a layout move or a
  restore. Mesail packs every root before every deploy (ADR 0004 section
  4), so every device would do a full resync after each deploy. Keep
  Mesail's own log unless both are solved. FYLO's browser `.vdbx`
  encryption with a host key (FYLO ADR 0025) and its Android AAR and C ABI
  (from F2) matter only if FYLO later replaces TACHYON's store behind the
  `DeviceStore` port (ADR 0006 section 1).

## Ownership check

| Path | Owner |
| --- | --- |
| `docs/**` (except `docs/mesail/ux/**`), `tests/blackbox/**`, `tools/crap/**`, `scripts/kit-sync.sh`, `CLAUDE.md`, `AGENTS.md`, `README.md`, `CHANGELOG.md`, `.gitignore` | architecture (this plan); slices append to `CHANGELOG.md` only under their own heading, and update CONTRACTS.md per rule 3 |
| `kit/<package>/**` and `apps/*/server/packages/<package>/**` | the slice that owns the package: I1 `oidc`, `identity` (Wave 3: L1 owns `kit/oidc` and `kit/addresses`, L5 the copies during the switch); M2 (from I2) `events`, `outbox`; D1 `recipients`, `directory`; C2 `channels`; A1 `blobs`, `attachments`; M1 `mime`, `mailauth`, `richtext`, `mailin`; M2 `compose`, `dkim`, `smtpclient`, `mailout`; S1 `textindex`, `search`; M3 `messaging`; R1 `realtime`; N1 `webpush`, `notify`. The workflow package of a service (rule 8b) is copied only into that service's app; `ids`, `collections` and `transport` stay with architecture |
| `apps/<service>/**` other files | the service's slice |
| `apps/web/client/**` | split between F1-F4 by the patterns above |
| `apps/web-admin/**` | F5 |
| `edge/smtp/**` | M1 |
| `edge/realtime/**` | R1 |
| `Dockerfile` (non-Keycloak stages), `docker-compose.yml`, `.dockerignore`, `.env.example`, `deploy/lambda/**`, `deploy/floci/**`, `deploy/router/**`, `scripts/floci/**`, `scripts/native/**` | P1 |
| `docs/mesail/ux/**` | `mesail-ux` |
| Wave 4 files (`DeviceStore*`, `Sync*`, `Queue*`, `Attention*`, `OfflineSearch*` in `apps/web/client/**`; the `changes` log in `apps/messaging`) | the Wave 4 slice named in "Wave 4: offline first" |
| `Dockerfile` Keycloak stages, `deploy/keycloak/**` (including the realm templates in `deploy/keycloak/realm/`) | `mesail-keycloak` |
| `.github/workflows/**`, `scripts/gates.sh`, `scripts/ci/**` | C1 |
| `scripts/vendor/**`, `tools/styles/**`, `tests/responsive/**` | F1 (C1 calls them from `scripts/gates.sh` for Tac apps) |
