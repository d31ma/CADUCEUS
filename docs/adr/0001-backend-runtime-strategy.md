# ADR 0001: Backend runtime strategy

- Status: Superseded by ADR 0002
- Date: 2026-07-23
- Superseded: 2026-07-26

## Context

CADUCEUS is a self-hosted, multi-domain mail server. Its application control
plane is implemented with Bun and Tachyon, while Fylo, Chex, and TTID run as
standalone native binaries. Fylo now provides clients for Go, Rust, and other
languages, so storage access no longer requires JavaScript.

The existing backend is nevertheless organized around Tachyon routes,
JavaScript feature services and repositories, shared authentication guards, and
Node-compatible SMTP, MIME, DKIM, ACME, and Web Push libraries. Most work is
network, filesystem, DNS, or child-process I/O.

## Decision

1. Bun, Tachyon, and incrementally strict TypeScript remain the primary backend
   stack for routes, authentication, authorization, business features,
   repositories, and Fylo access.
2. Tachyon remains the only process allowed to mutate the shared Fylo root.
3. Long-running mail services are supervised process resources. Enabled SMTP
   listeners and the outbound queue worker are required; failure terminates the
   application so the container supervisor can restart it. ACME renewal is
   optional and reports a degraded state when provisioning fails.
4. The outbound queue worker records run count, success, failure, overlap,
   duration, and last-error measurements. Runtime component state is emitted as
   structured, secret-free status logs.
5. Go may be introduced later as a standalone mail-transport data plane, not as
   per-feature or per-repository polyglot code. It must continue to call the
   signed local HTTP API or an equivalent authenticated local transport.
6. Rust is reserved for isolated, measured CPU-intensive or parser-security
   work. It is not the default application or mail-daemon language.

## Go extraction gate

A Go pilot starts only after representative production measurements establish
an operator-defined reliability, concurrency, latency, or memory objective that
the supervised Bun implementation cannot meet economically.

The first pilot must be one independently deployable boundary:

- inbound SMTP, or
- authenticated SMTP submission, or
- outbound queue scheduling and direct-to-MX delivery.

The pilot must preserve:

- Tachyon as the authorization and business-policy boundary;
- a single Fylo writer;
- HMAC-authenticated local requests;
- equivalent SMTP, MIME, DKIM, TLS, retry, bounce, and shutdown tests;
- rollback to the Bun implementation without a data migration.

## Consequences

- CADUCEUS avoids a high-risk rewrite while gaining explicit lifecycle and
  operational evidence.
- New backend TypeScript modules can be made strict incrementally without
  blocking on the existing checked-JavaScript backlog.
- A future Go daemon adds another build artifact and toolchain, so it is
  justified by measurements rather than preference.
- Tachyon's polyglot route support remains available for isolated handlers, but
  is not used to fragment core business features across languages.
