# ADR 0002: TypeScript Frontend and Go Backend

- Status: Accepted
- Date: 2026-07-26
- Supersedes: ADR 0001

## Context

CADUCEUS is a self-hosted, multi-domain mail server and browser mail client.
It owns long-running SMTP listeners, authenticated submission, direct-to-MX
delivery, durable retry queues, DNS and TLS behavior, authentication, mailbox
and domain authorization, and FYLO-backed metadata and attachment storage.

The original backend used Bun and Tachyon because FYLO initially required a
Node-compatible integration. FYLO now has a language-neutral executable
contract, and Tachyon can continue to provide the Tac development and frontend
build workflow without constraining the production backend language.

The decision is made as a greenfield architecture choice. Existing JavaScript
is migration input, not a reason to preserve the old runtime indefinitely.

## Decision

1. Write browser behavior in strict TypeScript using Tachyon/Tac and
   standards-based Web Components.
2. Build one long-running Go modular monolith for the production backend.
3. The Go process owns:
   - the public HTTP API and production static-client serving;
   - inbound SMTP and authenticated submission listeners;
   - outbound scheduling, retries, bounces, and direct-to-MX delivery;
   - authentication, authorization, tenant isolation, and administrative
     workflows;
   - the authoritative FYLO process lifecycle and all mutations;
   - health, readiness, metrics, structured logging, and graceful shutdown.
4. Use Tachyon as the frontend development server and production frontend
   builder. The production runtime does not require Tachyon unless an
   independently reviewed runtime capability makes it necessary.
5. Keep public HTTP contracts language-neutral and versioned. OpenAPI and JSON
   Schema artifacts are the contract between the TypeScript client and Go
   server.
6. Do not divide ordinary backend features among several languages. Rust is
   reserved for a separately tested hostile-input parser or measured hotspot
   when evidence justifies another toolchain.
7. Migrate incrementally behind observable contracts. Do not run two
   authoritative FYLO writers against one root.

## Process Model

Development:

```text
browser -> Tachyon/Tac dev server -> Go API on loopback
                                      |
                                      +-> one persistent FYLO process
                                      +-> SMTP/submission listeners
                                      +-> outbound queue workers
```

Production:

```text
browser -> caduceus HTTP/API/static server
                    |
internet SMTP ------+-> mail protocol and delivery services
                    |
                    +-> one persistent FYLO process -> FYLO root and buckets
```

The primary production artifact is a multi-architecture Linux OCI image. A
native archive becomes supported only after its install, lifecycle, recovery,
upgrade, and rollback gates pass on native infrastructure.

## Consequences

- The backend gains a runtime designed for long-lived network services,
  concurrency, bounded workers, and straightforward native packaging.
- The browser keeps the most direct ecosystem for DOM, accessibility, PWA, and
  Tachyon/Tac development.
- The production image can omit Bun, Node, and Tachyon after the frontend build,
  reducing its runtime dependency and attack surface.
- During migration, JavaScript routes remain compatibility adapters only.
  New domain behavior belongs in Go.
- The repository temporarily carries two backend implementations. Contract
  parity, one-writer ownership, feature flags, and reversible cutovers are
  mandatory until the legacy backend is removed.

## Rejected Alternatives

- **TypeScript/Bun for the complete backend**: productive and compatible with
  the current code, but not the preferred greenfield runtime for the
  continuously running mail data plane.
- **Rust for the complete backend**: strong safety and performance, but a higher
  implementation and contributor cost for the complete business application.
- **Polyglot backend by feature**: increases build, debugging, security,
  compatibility, and release complexity without a corresponding product
  benefit.
- **Big-bang rewrite**: delays observable value and makes parity, rollback, and
  data-safety failures harder to isolate.

## Acceptance Gate

This architecture is complete only when:

- all production backend behavior runs in the Go process;
- the TypeScript client passes the shared API and browser interaction corpus;
- the legacy JavaScript backend and its production dependencies are removed;
- one authoritative FYLO owner is enforced;
- SMTP, queue, attachment, authentication, tenant, backup, restore, upgrade,
  rollback, security, and native release evidence is published;
- the production image is built from pinned inputs, scanned, accompanied by an
  SBOM and provenance, and verified before promotion.
