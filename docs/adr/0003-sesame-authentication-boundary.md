# 3. SESAME owns authentication and authorization

Date: 2026-07-26

## Status

Accepted. Supersedes the hand-written authentication described in
[ADR 0002](0002-typescript-frontend-go-backend.md) as the Phase 3 plan.

## Context

Phase 3 was written as "migrate authentication into Go": rebuild passkeys,
passwords, TOTP, recovery codes, session issuance and revocation, domain
claims, roles, and the security audit trail as CADUCEUS code.

That is a large amount of security-critical code to own. The prior JavaScript
implementation was already about 1,150 lines across credential storage, a
WebAuthn verifier, a TOTP verifier, a hand-rolled HS256 JWT, and a policy
module — and it had no independent verification, no security ledger, and a
revocation story bounded by token lifetime.

[SESAME](https://sesame.del.ma) is a headless authentication and authorization
engine built on the same FYLO runtime CADUCEUS already depends on. It opens no
port. The host application starts it as a child process and speaks NDJSON over
its stdin and stdout, so it adds no listener, no admin console, and no origin
to confuse. The originally qualified `26.30.07` release, now superseded by
`26.33.02`, implements every item on the Phase 3 list:
Argon2id passwords, WebAuthn passkeys with clone detection, TOTP with durable
replay prevention, single-use recovery codes, revocable sessions, default-deny
authorization over roles and grants, OAuth 2.0 / OIDC with PKCE and rotating
refresh families, and a hash-chained security ledger.

## Decision

CADUCEUS does not implement authentication. One Go-owned SESAME process makes
every credential, session, token, and permission decision, and the rest of the
system reaches those decisions through one composed operation surface.

**One owner.** The Go runtime starts the engine, as it already starts FYLO. The
engine is qualified at startup against the digest and release identity pinned in
`runtime.lock.json`, the machine protocol version the client speaks, the exact
list of operations CADUCEUS calls, and storage readiness. Any gap is a startup
failure, not a failure during someone's login.

**Two FYLO roots.** SESAME writes its security ledger to its own FYLO root
inside its deployment directory. The mail store keeps its own. One FYLO process
owns one root, so the identity ledger and the mail data are separate writers
over separate data, and the project's single-writer invariant holds for both.

**Composed operations, not a proxy.** The legacy runtime reaches the boundary
through `auth.*` operations that each perform a complete step — begin a login,
supply a factor, issue a token set, authorize a request. Raw engine operations
are deliberately not forwarded. Orchestrating them *is* the authorization
boundary, so it lives in Go and nowhere else.

**Tokens are SESAME's.** A login issues a session, and the session is exchanged
for an ES256 access token through the ordinary authorization-code flow with
PKCE, run inside the process: CADUCEUS is both the authorization host and its
own first-party client, so no browser redirect is involved. The hand-rolled
HS256 JWT, its shared secret, and `JWT_SECRET` are removed.

**Every request is verified twice, for two different facts.** The signature is
checked locally against the engine's published keys, which proves the token was
minted by this deployment and not altered. The engine is then asked whether the
grant still stands, because revocation cannot be written into an
already-signed token. A session revoked one second ago fails on the next
request rather than when its token would have expired anyway.

**One tenant; domains are resources; authority is a group.** The deployment is
one SESAME tenant and every mailbox in every mail domain is a principal in it.
A domain is not a tenant because group membership requires the principal and
the group to share one, and domain-per-tenant would make a person who
administers two domains two identities with two logins. Instead each domain has
an admin and a viewer role, each granted once to a matching group, and
authority is membership. A mailbox may be a member of as many domain groups as
it administers and still signs in once; leaving one takes that domain's
authority and no other's. Resources stay `mailbox:<domain>:<local>` and
`domain:<domain>`, so a domain administrator holds `mailbox:<domain>:*` and a
mailbox owner holds exactly one mailbox.

## Consequences

**What improves.** The credential code CADUCEUS owns drops to zero. Revocation
and principal suspension take effect immediately rather than within a token
lifetime. Authorization is default-deny with a stable reason code and a policy
version on every answer. Refresh tokens rotate with reuse detection. There is a
hash-chained, rebuildable security ledger where there was none. Token
confusion, algorithm downgrade, replayed and forged tokens, cross-domain
access, and premature completion all fail closed, and each is a test that runs
against the released engine.

**What it costs.** CADUCEUS gains a second pinned runtime dependency and a
second supervised child process. Each authorized request makes two engine
round trips — one to confirm the grant stands, one to decide. These are pipe
operations against a local process and have not been measured as a bottleneck;
if they become one, the fix is a short-TTL cache keyed by session and policy
version that revocation invalidates, not skipping the check.

**What changes for operators.** The deployment directory holds the signing,
secrets, and snapshot keys and must be backed up and mounted durably: losing it
ends every session and invalidates every issued token. `caduceus auth init`
creates it and refuses to touch an existing one. Provisioning is explicit —
`caduceus auth domain`, `auth mailbox`, `auth role`, `auth credentials` — and a
new mailbox receives a password and a recovery code set, which is what lets its
owner reach MFA assurance once and enrol a passkey of their own.

**What we accept.** SESAME is a developer preview and no release has passed a
72-hour native gate or an independent security review. CADUCEUS pins one
qualified release by digest and fails closed on anything else. Phase 8 is where
production support is claimed, and this dependency is part of what has to be
qualified there.

**What the single tenant costs.** SESAME's tenant boundary no longer separates
mail domains. What separates them is what already did: every decision is
default-deny over a domain-qualified resource, and FYLO's per-domain UID/GID
ownership sits below any caller-controlled filter. Storage ownership follows
the domain a decision authorized rather than the caller's own address, so a
cross-domain administrator writes into the domain it is acting in. A deployment
that needs a hard tenant boundary between unrelated businesses should run
separate deployments, which is what a tenant boundary would have meant anyway.

**What the test suite gave up.** It can no longer mint a token. Every suite
starts the real Go owner, the released engine, and the released FYLO, then
provisions a mailbox and signs in with the credentials it is handed. A harness
that could forge a credential would prove nothing about the boundary that
verifies them, and a forged passkey record cannot exist at all now: the engine
will not accept a credential it did not register.
