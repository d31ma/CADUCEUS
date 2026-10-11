# CADUCEUS Enterprise Project Plan

## 1. Executive Decision

CADUCEUS will be developed as:

- a strict TypeScript Tachyon/Tac browser client;
- one long-running Go modular-monolith backend;
- FYLO, CHEX, and TTID as pinned native runtime dependencies;
- a self-contained Linux OCI image as the primary production artifact.

Tachyon remains the frontend development server and build tool. Production
serving moves to Go after `ty bundle` emits the versioned static client.

> **Status (2026-10-11):** the server has been rewritten in Go on Tachyon Yon
> and the repository runs on it; see [section 20](#20-go-rewrite-status). The
> production process differs from section 4: `caduceus entry` is PID 1 and
> runs `ty serve`, which serves the client and the Go handlers.

The migration is a strangler migration, not a rewrite branch that remains
unshippable for months. Each slice must preserve observable behavior, data
compatibility, rollback, security, and operator documentation.

## 2. Product Boundaries

CADUCEUS owns:

- multiple hosted mail domains in one deployment;
- principals, mailboxes, aliases, domain administration, and local
  authentication;
- inbound SMTP, authenticated submission, direct-to-MX delivery, DKIM, TLS,
  retries, bounces, suppression, and routing;
- message metadata, mailbox state, raw messages, and attachment bytes;
- the browser mail client, PWA behavior, realtime mailbox updates, and
  administrative UI;
- backup, restore, migration, audit, diagnostics, and release artifacts needed
  to operate the system.

The default installation must not require a proprietary cloud mail, identity,
database, queue, object-storage, analytics, or monitoring service.

The following are not v1 claims until independently implemented and proven:

- active-active writers or shared-network-filesystem data roots;
- exactly-once SMTP delivery;
- complete malware detection;
- every SMTP extension or internationalized-address profile;
- production support on Windows or macOS;
- inbox placement guarantees at remote providers.

SMTP is fundamentally at-least-once across ambiguous network failures.
CADUCEUS must minimize and identify duplicates without claiming impossible
exactly-once delivery.

## 3. Non-Negotiable Invariants

- One Go process owns one authoritative FYLO root and persistent machine
  process.
- An inbound message is acknowledged with SMTP `250` only after the accepted
  envelope, raw message, metadata, and required queue state are durable.
- An outbound item is removed only after a durable terminal outcome is
  recorded.
- Tenant/domain and mailbox ownership are enforced below caller-controlled
  filters.
- Missing or ambiguous authentication, authorization, domain, or storage state
  fails closed.
- Raw messages and attachments are treated as hostile bytes.
- Secrets, credentials, recovery material, private keys, message bodies, and
  attachment contents never enter ordinary logs, traces, or metrics.
- Every queue, parser, body, attachment, recipient list, retry schedule,
  goroutine pool, subprocess frame, and external read has an explicit bound.
- Schema changes are versioned, resumable, observable, safe to retry, and
  backed by a restore-tested rollback plan.
- Public APIs, stored data, SMTP behavior, configuration, and release artifacts
  have explicit compatibility policies.

## 4. Target Process Architecture

### Development

`ty serve` provides frontend routing and reload. The Go backend runs on a
separate loopback port. Development-only routing or CORS configuration connects
the two; it must never become a privileged production bypass.

```text
client TypeScript -> Tachyon/Tac dev server
                              |
                              +-> Go /api/v1 on loopback
                                      |
                                      +-> FYLO machine process
                                      +-> SMTP listeners
                                      +-> delivery workers
```

### Production

The `caduceus` Go executable is PID 1 in the container. It serves the bundled
client and API, owns mail listeners and workers, supervises FYLO, handles
signals, and reports component health.

```text
HTTPS client ----------> Go HTTP/API/static server
SMTP peers ------------> Go SMTP receiver
mail clients ----------> Go submission server
Go delivery workers ---> DNS/MX/TLS remote SMTP
Go persistence --------> persistent FYLO machine process
FYLO ------------------> metadata root and attachment buckets
```

Tachyon, Bun, compilers, package managers, and test tools stay in build stages.
They are not copied into the production image.

### Runtime Ownership

- HTTP readiness does not report ready until configuration, keys, schemas,
  writer ownership, and FYLO recovery checks pass.
- Liveness proves the process can make progress; it does not depend on public
  DNS or a remote mail provider.
- A required listener or FYLO failure terminates the process after bounded
  cleanup so the container or service manager can restart it.
- Optional integrations report degraded status without granting access or
  losing accepted mail.
- Shutdown stops admission, drains bounded in-flight work, checkpoints durable
  state, closes listeners, stops children, and exits within a configured
  deadline.

## 5. Enterprise Repository Shape

Create a directory only with its first working vertical slice. Empty
architecture placeholders are prohibited.

```text
.
├── cmd/
│   └── caduceus/                  # Go executable composition root
├── internal/
│   ├── app/                       # lifecycle and dependency composition
│   ├── auth/                      # authentication, sessions, recovery
│   ├── domains/                   # hosted domains, aliases, routing policy
│   ├── users/                     # principals, mailbox membership, preferences
│   ├── mailbox/                   # folders, threads, search, state changes
│   ├── message/                   # envelope, headers, MIME and raw-message model
│   ├── delivery/                  # durable outbound state machine and bounces
│   ├── inbound/                   # SMTP receive and routing
│   ├── submission/                # authenticated SMTP submission
│   ├── audit/                     # security and administrative audit events
│   ├── adapters/
│   │   ├── fylo/                  # FYLO process, documents, buckets, migrations
│   │   ├── dns/                   # resolver and MX policy
│   │   ├── smtp/                  # wire adapters and remote delivery
│   │   ├── tls/                   # certificate and TLS policy
│   │   └── sentdm/                # optional integration boundary
│   └── platform/
│       ├── config/
│       ├── buildinfo/
│       ├── clock/
│       ├── crypto/
│       ├── lifecycle/
│       ├── observability/
│       └── webui/                 # production static asset serving
├── api/
│   ├── openapi/
│   │   └── v1.yaml
│   ├── schema/                    # shared JSON Schemas
│   ├── events/                    # SSE/event contracts
│   └── fixtures/                  # canonical request/response corpus
├── client/
│   ├── components/                # Tachyon/Tac TypeScript components
│   ├── pages/
│   └── shared/
│       ├── assets/
│       ├── data/
│       ├── scripts/
│       ├── styles/
│       └── types/
├── db/
│   ├── schemas/                   # FYLO schema history and manifests
│   ├── migrations/                # resumable migration definitions
│   └── seed/                      # explicit development seed recipes
├── test/
│   ├── blackbox/
│   ├── contract/
│   ├── fylo/
│   ├── smtp/
│   ├── interop/
│   ├── adversarial/
│   ├── chaos/
│   ├── performance/
│   ├── soak/
│   └── fixtures/
├── tests/
│   └── e2e/                       # Playwright browser interaction/visual matrix
├── tools/
│   ├── contracts/                 # reproducible contract generation and drift
│   ├── coverage/                  # package-group coverage ratchet
│   ├── crap/                      # maximum CRAP score gate
│   ├── runtime-lock/              # binary manifest verification
│   └── production-evidence/       # restore, upgrade, rollback and soak runner
├── deploy/
│   ├── container/
│   ├── compose/
│   ├── systemd/                   # added only when native Linux is supported
│   └── examples/
├── docs/
│   ├── adr/
│   ├── rfcs/
│   ├── operations/
│   ├── security/
│   └── standards/
├── .github/
│   ├── CODEOWNERS
│   ├── dependabot.yml
│   └── workflows/
├── go.mod
├── go.sum
├── package.json                   # frontend development dependencies only
├── bun.lock
├── runtime.lock.json              # exact ty/FYLO/CHEX/TTID artifacts and digests
└── VERSION
```

Do not create `utils`, `helpers`, `common`, `shared`, `misc`, `base`, or
`manager` dumping grounds in Go. Packages are named after domain
responsibilities. Small interfaces are owned by their consumer; adapters
implement them.

## 6. Dependency Direction

```text
cmd/caduceus
  -> internal/app
      -> domain/application feature packages
          -> consumer-owned ports
              <- adapters/fylo|dns|smtp|tls
      -> platform lifecycle/config/observability

client
  -> generated public API types/client
      -> /api/v1
```

Rules:

- domain packages do not import HTTP, Tachyon, FYLO, SMTP wire, filesystem, or
  observability implementations;
- HTTP and SMTP adapters translate bounded input and call feature services;
- adapters never decide authorization or business policy;
- cross-feature calls use deliberate service contracts rather than importing
  storage internals;
- exported Go symbols are intentional contracts and are documented;
- `context.Context` is first for blocking or cancellable operations and is
  never stored in long-lived objects;
- clocks and deterministic identifiers are injectable for tests, but
  production cryptographic randomness is not caller-substitutable.

## 7. Public Contract Strategy

OpenAPI 3.1 and shared JSON Schemas become the canonical HTTP contract.
Generation produces:

- Go request/response types and server-interface assertions;
- strict TypeScript request/response types and a small fetch client;
- Tachyon development schemas where needed;
- documentation examples and a compatibility corpus.

Generated output carries its source and generator version and is checked for
drift in CI. Human-readable error messages are not parsing contracts. Errors
have:

```json
{
  "code": "stable_machine_code",
  "message": "safe summary",
  "retryable": false,
  "request_id": "opaque correlation id",
  "details": {}
}
```

API v1 is additive by default. A breaking change requires an RFC, migration,
deprecation window, compatibility tests, and a new major contract version.
Unknown-field and enum-extension behavior is specified. Every mutation defines
idempotency and retry safety.

SSE event types are versioned independently. Clients recover from cursor loss,
duplicate delivery, reconnect, and unknown additive event fields.

## 8. FYLO and Data Plan

### Authority

The Go process owns one persistent FYLO machine client and an exclusive writer
lease for the root. All legacy mutations must pass through this owner before
any Go feature is promoted. A second owner refuses startup.

### Tenant Isolation

Record an ADR for FYLO identity mapping before migration. The proposed mapping
is:

- domain/tenant ownership -> FYLO `gid`;
- mailbox or service principal -> FYLO `uid`;
- deployment maintenance -> an explicit, audited system identity.

The proving suite must confirm the exact FYLO semantics. Application checks and
FYLO uid/gid enforcement are defense in depth; neither replaces the other.
Tenant identity is included in every tenant-bound document, key, query, cache
key, event, metric label policy, and authorization decision.

### Messages and Attachments

- Store searchable mailbox/message metadata as FYLO documents.
- Store raw RFC message bytes and attachment bytes in FYLO buckets.
- Persist content length, media type, digest, disposition, safe filename, and
  ownership metadata separately.
- Stream uploads/downloads; never require the whole object in memory.
- Verify the digest and ownership on read.
- Use opaque storage keys; user filenames never become filesystem paths.
- Deleting metadata and bytes is a durable, retry-safe workflow with orphan
  detection and repair.

### Schema and Migration

- Keep schema history and upgraders under `db/`.
- Seed recipes are explicit and never copied into production state.
- No default production administrator, password, token, or private key ships.
- Before migration: verify backup, free space, runtime identity, schema range,
  and exclusive ownership.
- Migrations checkpoint progress, are safe to resume, and emit bounded status.
- Every supported prior data version has an upgrade fixture.
- Rollback is proven where old code can safely read new state; otherwise the
  release declares a restore-required rollback boundary.

## 9. Migration Roadmap

### Phase 0: Preserve and Specify the Baseline

Status: **Complete — 2026-07-26**

Deliver:

- checkpoint the current work without destructive Git operations;
- record all existing known test failures separately from migration failures;
- freeze HTTP, SMTP, data, authentication, and UI behavior as black-box
  fixtures;
- accept ADR 0002 and mark ADR 0001 superseded;
- define API v1, configuration, stable errors, runtime lock, compatibility, and
  support vocabulary;
- publish the threat model and migration rollback policy.

Evidence:

- the non-destructive checkpoint command and recovery manifest are defined in
  `scripts/checkpoint-phase0.mjs`;
- HTTP routes, stable errors, configuration, runtime binaries, and phase parity
  are frozen under `contracts/` and `runtime.lock.json`;
- authentication, SMTP, persistence, FYLO isolation, browser interaction, and
  responsive visual behavior are exercised by the baseline test corpus;
- ADR 0002 is accepted and ADR 0001 is explicitly superseded;
- `docs/BASELINE.md`, `docs/THREAT_MODEL.md`, and
  `docs/MIGRATION_ROLLBACK.md` record the operational evidence and policies;
- `bun run verify:phase0:all` is the complete local qualification gate, while
  CI runs its core and browser halves as separate jobs.

Exit:

- [x] the current application can be built and exercised reproducibly;
- [x] every migration slice has an external parity test;
- [x] there is no ambiguity about production ports, ownership, or artifact model.

### Phase 1: Go Runtime Foundation

Status: **Complete — 2026-07-26**

Build one minimal vertical slice:

- `caduceus version`, `doctor`, `serve`, and `migrate`;
- strict configuration with safe defaults and unknown-key rejection;
- build metadata, request IDs, JSON logs, metrics, health, and readiness;
- signal handling and bounded graceful shutdown;
- a versioned `/api/v1/system` contract;
- frontend build copied into a Go-served static directory.

Exit:

- [x] Go serves the bundled login shell and system endpoint;
- [x] invalid configuration and missing runtime dependencies fail before listeners
  open;
- [x] no production behavior has silently changed.

Evidence:

- `docs/PHASE1_RUNTIME.md` freezes the CLI, configuration, listener, startup,
  shutdown, and static-bundle acceptance contract;
- `docs/PHASE1_EVIDENCE.md` maps each exit criterion to its tests and records
  the completed qualification results;
- `bun run verify:phase1:all` is the complete Phase 0 compatibility plus Phase
  1 Go runtime gate;
- CI blocks on the Phase 0 baseline, Phase 1 Go runtime, and browser jobs.

### Phase 2: FYLO Viability and Single-Writer Cutover

Status: **Complete — 2026-07-26**

Build a disposable proving harness before migrating domain features:

- persistent process protocol, framing, stderr draining, cancellation, restart,
  timeouts, and version/capability checks;
- exact runtime artifact and digest validation from `runtime.lock.json`;
- exclusive root ownership;
- uid/gid tenant and mailbox isolation;
- bucket streaming, digest validation, orphan repair, and quotas;
- crash injection around writes and acknowledgements;
- index deletion/rebuild, corruption detection, cold backup/restore, and
  migration retry;
- bounded mixed read/write load and leak measurement.

Then route legacy persistence through the Go owner using a temporary,
authenticated, allowlisted local bridge. Do not expose raw FYLO operations as a
public API.

Exit:

- [x] JavaScript and Go cannot become concurrent FYLO writers;
- [x] restore reproduces tenant access decisions and attachment integrity;
- [x] measured limits and unsupported topologies are documented.

Evidence:

- `docs/PHASE2_FYLO.md` freezes the persistent-process, bridge, isolation,
  bucket, reconciliation, and cold-restore contracts;
- `docs/PHASE2_EVIDENCE.md` records the released-runtime identity, measured
  limits, supported topology, failure injection, Docker smoke test, coverage,
  and upstream capability-reporting issue;
- `bun run verify:phase2:all` is the complete compatibility, browser, Go
  foundation, released-FYLO, recovery, and single-writer gate;
- CI installs the digest-locked FYLO 26.33.02 release and blocks on the Phase 2
  qualification job.

### Phase 3: Authentication and Tenant Boundary

Status: **Complete** — see `docs/PHASE3_EVIDENCE.md`.

CADUCEUS does not implement authentication. One Go-owned SESAME engine makes
every credential, session, token, and permission decision. See
`docs/adr/0003-sesame-authentication-boundary.md` for why, and
`docs/PHASE3_AUTH.md` for the contract.

Adopt:

- SESAME release `26.33.02`, pinned by digest in `runtime.lock.json` and
  qualified at startup on release identity, protocol version, the exact
  operation list CADUCEUS calls, and storage readiness;
- one engine process owned by the Go runtime, writing its security ledger to
  its own FYLO root, never the mail root;
- composed `auth.*` operations over the existing authenticated loopback bridge;
  raw engine operations are not forwarded;
- SESAME-issued ES256 access tokens, obtained by exchanging a session through
  the in-process authorization-code flow with PKCE, with rotating refresh
  families;
- one deployment tenant, one mailbox per principal, and authority over a mail
  domain carried by membership of that domain's SESAME group, so one identity
  administers any number of domains with one login.

Remove:

- every hand-written credential path: password verifier, WebAuthn ceremony,
  TOTP verifier, recovery codes, MFA and setup sessions, auth audit records;
- the hand-rolled HS256 JWT, `JWT_SECRET`, and `AUTH_PASSWORD_PEPPER`;
- `CADUCEUS_AUTH_STEP_UP_MAX_AGE_SECONDS`, replaced by a freshly satisfied
  authentication transaction rather than a timestamp;
- the test-only forged passkey record; a credential the engine did not register
  cannot be accepted, and the test suite signs in with a recovery code.

Exit:

- malformed, replayed, expired, revoked, suspended, concurrent, and
  cross-domain cases fail closed, each proven against the released engine;
- an ID token is refused where an access token is required, and an
  algorithm-downgraded or claim-tampered token is refused;
- the TypeScript client and Go server pass one authentication contract corpus;
- no later feature can bypass the Go authorization boundary: every route
  reaches a mailbox only through a decision, and the decision is default-deny.

### Phase 4: Mailbox and Attachment Vertical Slices

Sequence:

1. read-only inbox, folders, threads, search, and raw-message retrieval;
2. attachment list/download and bucket integrity;
3. read/unread, star, move, archive, trash, snooze, and permanent delete;
4. templates, signatures, rules, preferences, and presence;
5. compose draft and scheduled-send persistence.

For each slice:

- shadow-read old and new implementations against the same fixture;
- compare normalized public responses;
- switch one contract behind a reversible feature flag;
- keep one writer;
- remove the legacy path only after production-equivalent tests pass.

### Phase 5: Outbound Delivery

Implement a durable state machine:

```text
accepted -> queued -> resolving -> connecting -> transmitting
         -> delivered | deferred -> retrying | bounced | cancelled
```

Each transition records attempt number, next-attempt time, safe diagnostic,
remote response class, and correlation identity without logging message bodies
or credentials. Retry policy is bounded and deterministic. Ambiguous outcomes
remain visible to operators and may produce a duplicate; they are never
silently marked delivered.

Exit:

- compose-to-delivery works through the Go API and queue;
- DKIM canonicalization/signing, DNS/MX policy, TLS policy, bounces,
  suppression, scheduled send, undo, and restart behavior pass;
- accepted queued mail survives forced termination and restore.

### Phase 6: Inbound SMTP and Submission

Submission authentication is refused until this phase issues application
passwords: SMTP AUTH carries one secret and a session requires multi-factor
assurance, so the credential must be a SESAME workload principal holding only
`mailbox:send` for one mailbox, revocable without touching the human's
credentials. See `docs/PHASE3_EVIDENCE.md`.

Implement:

- SMTP command/envelope state machine and advertised-extension registry;
- hosted-domain and recipient validation;
- bounded DATA streaming and raw-message durability;
- local routing, rules, aliases, and mailbox delivery;
- authenticated submission with TLS and abuse controls;
- explicit relay prevention;
- structured SMTP session diagnostics without message content.

Exit:

- relay, smuggling, injection, oversized input, slow-client, malformed MIME,
  duplicate-recipient, cross-domain, and quota cases are covered;
- Postfix, Exim, and test-client interoperability passes for the claimed
  extension profile;
- the server never returns successful acceptance before durability.

### Phase 7: Production HTTP Cutover

- Point the TypeScript client directly at the Go `/api/v1`.
- Serve `ty bundle` output from Go.
- Remove Tachyon production route adapters.
- Remove Bun/Node and JavaScript backend dependencies from the runtime image.
- Delete legacy server code only after import/build/contract evidence proves it
  is unreachable.

Exit:

- Tachyon is required for development/build only;
- the production image contains only the Go application, static client, pinned
  FYLO runtime set, certificates/licenses, and necessary OS trust data;
- rollback to the last legacy release remains documented and restore-tested.

### Phase 8: Production Qualification

- complete native Linux amd64 and arm64 validation;
- run remote backup and cold restore;
- run previous-to-current upgrade and safe rollback fixtures;
- run at least a 72-hour release-profile soak;
- complete protocol conformance for every claimed mail extension;
- complete an independent security review;
- publish measured capacity, RPO/RTO, limitations, and support window.

Only then may the project call a release production-supported.

## 10. Test and Evidence Strategy

### Fast Developer Gate

Frontend:

```sh
bun install --frozen-lockfile
bun run typecheck
bun test
ty bundle
```

Backend:

```sh
gofmt -w cmd internal test tools
go test ./...
go vet ./...
go build -trimpath ./cmd/caduceus
```

Run the smallest package or browser scenario first.

### Pull-Request Gate

- strict TypeScript checking and Tachyon bundle;
- Go formatting, tests, vet, staticcheck, and race detector;
- package-group coverage floors and CRAP score ceiling of 15;
- generated contract and documentation drift;
- real pinned FYLO integration;
- API and stable-error compatibility;
- authentication and cross-tenant adversarial suite;
- SMTP state-machine and golden raw-message corpus;
- Playwright Chromium/Firefox/WebKit interaction tests;
- responsive visual matrix, keyboard navigation, reduced motion, and
  accessibility checks;
- production container build, non-root startup, health/readiness, and no
  test-route/seed/secret checks;
- dependency, license, secret, and vulnerability scans.

### Scheduled Gate

- Go fuzzing for SMTP commands, addresses, headers, MIME, search, routing,
  configuration, JSON, and DKIM canonicalization;
- extended browser/device matrix;
- SMTP interoperability matrix;
- race and high-concurrency queue tests;
- FYLO crash, cancellation, corruption, permission-loss, disk-full, rebuild,
  backup, and restore tests;
- DNS timeout, NXDOMAIN, SERVFAIL, rebinding, private-range, MX fallback, and
  TLS failure simulations;
- certificate expiry/renewal, clock skew, and key rotation;
- dependency freshness and OpenSSF Scorecard;
- short performance and leak profiles.

### Release Gate

- all PR and scheduled checks on the exact candidate commit;
- native Linux amd64/arm64 package and container tests;
- migration from every supported stored-data version;
- cold restore preserving messages, attachments, sessions, revoked access, and
  queue state;
- downgrade/rollback proof or an explicit restore-required boundary;
- 72-hour mixed HTTP/SMTP/delivery soak on named reference hardware;
- p50/p95/p99 latency, queue age/depth, throughput, heap, goroutines, file
  descriptors, child processes, and disk growth within published limits;
- image and binary vulnerability scans;
- SPDX SBOM, checksums, provenance, and attestation verification;
- independent security assessment before the first supported release.

Flaky tests do not get silently skipped. The claim they protect is blocked or
downgraded until deterministic evidence exists.

## 11. Mail-Specific Security Program

Maintain `docs/THREAT_MODEL.md` with at least these trust boundaries:

1. untrusted SMTP peer to inbound listener;
2. authenticated mail client to submission listener;
3. browser to HTTP/API edge;
4. Go application to FYLO process and data root;
5. application to DNS resolvers and remote MX hosts;
6. operators and administrators to privileged commands;
7. release artifacts and update channel.

Required controls include:

- default-deny relay and authorization;
- constant-time secret verification where observable;
- separate key storage for session, encryption, DKIM, and TLS private keys;
- SMTP command, line, recipient, message, header, MIME depth, part, and
  attachment limits;
- SMTP smuggling and line-ending ambiguity tests;
- header injection and address-normalization tests;
- DNS rebinding and private-network egress defense for attacker-controlled
  destinations;
- sandboxed HTML-message rendering, scripts disabled, remote content off by
  default, safe URL handling, and strict CSP;
- opaque attachment keys, safe content disposition, MIME sniffing defense, and
  download isolation;
- uniform authentication errors, brute-force controls, passkey-first flows,
  revocable sessions, TOTP, and recovery codes;
- audit records for authentication, domain, user, route, key, migration,
  restore, and administrative changes;
- secret-canary tests across logs, traces, crash output, CI artifacts, and
  release archives.

No unsupported anti-malware or deliverability guarantee appears in marketing
or operator documentation.

## 12. Observability and Operability

Expose:

- `/healthz` for process liveness;
- `/readyz` for configuration, key, schema, writer, and FYLO readiness;
- a protected or loopback metrics endpoint;
- JSON logs with timestamp, level, component, event, request/session ID, domain
  identifier where safe, and stable error code;
- OpenTelemetry-compatible traces across HTTP, persistence, queue, DNS, and
  outbound attempts without message content.

Minimum operational metrics:

- inbound sessions, accepted/rejected recipients, accepted bytes;
- queue depth, oldest age, attempts, deferred, bounced, delivered, ambiguous;
- DNS and remote SMTP latency/result classes;
- HTTP latency/status by normalized route;
- authentication success/failure/rate-limit/recovery counts;
- FYLO latency, restarts, errors, root size, bucket bytes, and orphan count;
- active goroutines, file descriptors, memory, CPU, and disk growth;
- TLS/DKIM key age and certificate expiry.

Every alert has a runbook. Required runbooks cover queue growth, remote
delivery failure, disk pressure, FYLO corruption/unavailability, certificate
expiry, DNS failure, authentication abuse, suspected tenant escape, backup
failure, restore, key rotation, and release rollback.

Initial service objectives are established by the proving ground rather than
invented in advance. Two invariants are immediate:

- zero acknowledged inbound messages lost after restart;
- zero unauthorized cross-tenant reads or writes.

All other latency, throughput, RPO, RTO, and capacity claims include hardware,
dataset, platform, runtime versions, methodology, and date.

## 13. CI Workflow Design

### `ci.yml`

Runs on every pull request and default-branch push:

- change-scope and generated-drift checks;
- frontend typecheck, unit tests, and bundle;
- Go unit, vet, staticcheck, race, coverage, and CRAP checks;
- real FYLO integration;
- API/auth/SMTP contract and adversarial suites;
- browser E2E and accessibility;
- production image build and smoke test.

Use a final `required` aggregation job so branch rules depend on one stable
check name while individual jobs remain diagnosable.

### `security.yml`

Runs on pull requests, default-branch pushes, and weekly:

- CodeQL for Go and TypeScript/JavaScript;
- dependency review;
- `govulncheck`;
- frontend dependency audit;
- container and filesystem scanning;
- secret scanning and push protection;
- license policy;
- OpenSSF Scorecard on its supported trigger.

### `nightly.yml`

- fuzz budget with retained regression corpus;
- SMTP interoperability;
- chaos/recovery scenarios;
- extended browsers;
- native arm64 where available;
- performance regression and leak detection.

### `release-candidate.yml`

Runs from the reviewed `release/<version>` branch:

- verifies `VERSION`, contract version, changelog, runtime lock, and migration
  range;
- repeats all blocking checks;
- builds the frontend once and embeds/copies the exact output into release
  artifacts;
- builds an immutable candidate OCI image tagged by commit SHA;
- emits the release manifest, checksums, SBOM, provenance, and attestations;
- verifies every generated attestation before candidate promotion.

### `release.yml`

Runs only after the release PR is merged and the candidate digest has passed
release evidence:

1. promote the existing candidate digest; do not rebuild;
2. publish immutable `:<version>` and moving `:stable` tags;
3. verify pull-by-digest startup and migrations in a clean environment;
4. create and push `v<version>` on the exact merged commit;
5. create the GitHub Release with manifest, checksums, SBOM, migration notes,
   support tier, and verification commands;
6. retain the `release/<version>` branch.

No pull request from a fork receives write credentials. Workflow permissions
are read-only by default and widened per job. All third-party actions are pinned
to full commit SHAs.

## 14. Release Artifact Model

### Versioning

Continue CADUCEUS CalVer:

```text
YY.WW.DD
YY.WW.DD-1
YY.WW.DD-2
```

All components derive the release date, ISO week, weekday, and same-day
sequence from UTC. `VERSION`, image label, manifest, git tag, and GitHub Release
must agree.

### Primary Artifact

Publish:

- `ghcr.io/d31ma/caduceus:<version>`;
- `ghcr.io/d31ma/caduceus:stable`;
- immutable digest references.

Initial production candidates:

- Linux amd64;
- Linux arm64.

macOS may be a developer build. Windows is unsupported until FYLO and all mail,
filesystem, process, backup, restore, and upgrade behavior passes natively.

### Release Manifest

Record:

- CADUCEUS version, commit, build time policy, and Go toolchain;
- Tachyon build version and digest;
- FYLO, CHEX, and TTID versions, protocols, targets, and digests;
- frontend lock digest and contract version;
- supported data-schema range;
- image platform digests;
- licenses, SBOM digest, provenance identity, and verification commands;
- support tier and known limitations.

Builds use clean ephemeral runners, pinned base-image digests, `-trimpath`,
deterministic metadata, and `CGO_ENABLED=0` where the selected libraries permit.
Build secrets use secret mounts, never build arguments.

## 15. Repository and Review Controls

- Protect the default branch; no direct pushes.
- Require the aggregated CI, security, and review checks on the current head.
- Require conversations resolved and stale approvals dismissed.
- Require CODEOWNERS review for:
  - auth and security;
  - mail protocol and delivery;
  - FYLO schemas, migrations, backup, and restore;
  - workflows, Dockerfiles, runtime locks, and release tooling.
- Enable private vulnerability reporting.
- Enable secret scanning and push protection.
- Require immutable full-SHA GitHub Actions references.
- Use Dependabot weekly for Go modules, Bun/npm, Docker, and GitHub Actions;
  the current text `bun.lock` is supported.
- Group routine patch/minor updates by ecosystem; security updates remain
  independently reviewable.
- Never auto-merge runtime, crypto, mail-parser, FYLO, Tachyon, or release-tool
  changes without their full affected test matrix.

Every material pull request states:

1. behavior and invariant changed;
2. public, stored-data, SMTP, and configuration compatibility;
3. failure and rollback behavior;
4. tests and native platforms used as evidence;
5. security, privacy, and operational impact;
6. documentation and migration updates.

## 16. Documentation Set

The public root must contain:

- `README.md`;
- `LICENSE`;
- `SECURITY.md`;
- `SUPPORT.md`;
- `CONTRIBUTING.md`;
- `CODE_OF_CONDUCT.md`;
- `GOVERNANCE.md`;
- `CHANGELOG.md`;
- `VERSION`;
- `runtime.lock.json`.

The maintained documentation set is:

- `docs/PROJECT_PLAN.md`;
- `docs/ARCHITECTURE.md`;
- `docs/ENGINEERING_STANDARDS.md`;
- `docs/THREAT_MODEL.md`;
- `docs/RELEASE_ENGINEERING.md`;
- `docs/PRODUCTION_EVIDENCE.md`;
- `docs/CONFIGURATION.md`;
- `docs/API.md`;
- `docs/DATA_MODEL.md`;
- `docs/DNS_AND_DELIVERABILITY.md`;
- `docs/SECURITY_HARDENING.md`;
- `docs/BACKUP_RESTORE.md`;
- `docs/UPGRADE.md`;
- `docs/LIMITATIONS.md`;
- `docs/operations/*.md`;
- `docs/adr/*.md`;
- `docs/rfcs/*.md`.

ADRs record hard-to-reverse choices. RFCs define proposed public behavior.
Runbooks describe detection, diagnosis, mitigation, recovery, and verification.
Examples are executable and CI-tested. Generated API docs and configuration
reference fail CI when stale.

Support language follows the SESAME evidence model:

- **buildable**: compilation succeeds;
- **native-tested**: tests pass on the native target;
- **preview**: distributed with explicit missing gates;
- **supported**: packaging, security, contract, SMTP interoperability, crash,
  restore, upgrade, rollback, soak, and operator gates all pass.

## 17. Definition of Done

A vertical slice is complete only when:

- the external behavior and failure modes are specified;
- a failing observable test existed before implementation;
- domain, transport, persistence, authorization, audit, and UI layers are
  complete where applicable;
- malformed, unauthorized, cross-tenant, concurrent, restart, and resource
  boundary cases pass;
- public contracts and generated clients are synchronized;
- migrations and rollback are proven;
- metrics, safe logs, alerts, and runbooks exist;
- user, operator, API, configuration, and changelog documentation is current;
- no dead legacy path, debug artifact, test route, sample secret, or unowned
  TODO ships;
- the exact affected CI and release gates pass.

## 18. Immediate Execution Order

1. Stabilize and checkpoint the current working tree.
2. Add the Go module, `VERSION`, `runtime.lock.json`, engineering standards,
   threat model, release policy, and canonical contract directories.
3. Implement the Go runtime/system vertical slice and frontend build handoff.
4. Complete the FYLO single-writer and uid/gid/bucket proving ground.
5. Cut authentication and tenant authorization to Go.
6. Migrate mailbox and attachment slices.
7. Migrate outbound delivery.
8. Migrate inbound SMTP and submission.
9. switch production HTTP/static serving to Go and remove the legacy backend.
10. Complete production evidence and promote the first supported release.

Do not begin Phase 3 until the Phase 2 one-writer, recovery, and tenant-isolation
gates pass. Do not begin Phase 4 until the Phase 3 boundary cannot be bypassed. Do not claim production support until Phase 8 passes.

## 19. SESAME Patterns Adopted and Adapted

Adopted:

- Go `cmd`/`internal` separation and a small dependency surface;
- domain vocabulary and non-negotiable invariants in repository instructions;
- ADRs for irreversible choices and RFCs for public contracts;
- canonical schemas and binary-backed contract corpora;
- adversarial, FYLO, native, restore, upgrade, rollback, and soak evidence;
- explicit buildable/native-tested/preview/supported vocabulary;
- pinned CI actions, coverage ratchets, race/static/vulnerability gates;
- checksums, SBOMs, provenance, verified attestations, and immutable artifacts;
- honest limitations and support claims.

Adapted for CADUCEUS:

- the Go process owns network listeners because mail is the product data plane;
- Linux OCI images are primary rather than a broad desktop executable matrix;
- Tachyon remains the TypeScript frontend development/build environment;
- mail protocol, MIME, DNS, TLS, queue durability, browser safety, and
  deliverability diagnostics add dedicated evidence;
- no broad language SDK matrix is needed; the browser client and documented
  HTTP/SMTP contracts are the public integration surfaces.

## 20. Go Rewrite Status

As of 2026-10-11 the server, the container entry point and the mail services
are Go. Layout and conventions: [`docs/GO_PORT.md`](./GO_PORT.md).

Done:

- **Server.** Every former route is a Go controller in `server/` (Tachyon Yon,
  package main), with the route logic in `server/shared/`; documents, JSON
  shapes, errors and token formats are unchanged. The route suites run it over
  HTTP through `ty serve`.
- **Entry point.** `caduceus entry` (`internal/runtime/entry`) replaces
  `scripts/entry.mjs`: it starts the Go FYLO owner, creates the collections,
  writes the handler settings to `.env`, runs `ty serve`, and supervises the
  inbound MX, submission, ACME and the queue, snooze, reminder and
  provider-notification workers. The Docker image starts it directly.
- **Mail services.** The inbound MX, submission, SPF/DKIM/DMARC, ACME and the
  tick workers are Go under `internal/`; the JavaScript daemons in
  `background/` are deleted. Inbound SMTP into the Go server's webhook is
  covered end to end by `tests/routes/inbound-smtp.test.js`.
- **JavaScript server.** Kept only under `tests/support/legacy-server/`, as the
  behavioural reference, the suites' seeding container and the record code the
  `admin:create` and `domain:migrate` scripts still use (Bun stays in the image
  for those two). Nothing else runs it.
- **Tests.** Route suites and the Playwright server (`scripts/e2e-preview.mjs`,
  one origin on `localhost:19875`) run the Go server.

Open:

- **Client.** The browser client is being merged separately (Go on Tachyon
  Tac); its behaviour and the Playwright assertions belong to that merge.
- **Tachyon release.** The Go server uses Tachyon features that are only on
  develop (binary Go bodies, `SocketSession.Request()`); the image and CI pin
  26.36.01, and the image can take an unreleased `ty` through
  `--build-context tachyon-override=<dir>`. A release is needed before the
  image builds from pinned versions alone.
- **Visibility rule.** `server/application.json` turns TY1333 off because it
  does not count calls from `server/shared/` route types.

## 21. Follow-ups

Accepted by Samuel 2026-10-11:

- Calendar offline write queue dropped in the Go frontend (failed save shows an error). Restore later.
- vCard imports over 64 KiB refused in the Go frontend. Lift later.
- Settings: new account, badge, sound and mailbox-name changes reach the sidebar only on the next page load (accepted on the calcon precedent).
- Settings: extension packages capped at 60 KiB (was 192 KiB). Lift later.
- Error toasts use fixed text (calcon, settings).
- Shell vendor DuVay provenance placeholder (0.0.0, UNLICENSED): real is DuVay 26.35.01-1, MIT (DELMA/DUVAY). Fix at frontend merge.
