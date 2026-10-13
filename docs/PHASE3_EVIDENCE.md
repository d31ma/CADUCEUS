# Phase 3 — Qualification evidence

Status: complete.

Re-qualified on macOS arm64, 2026-08-22, against SESAME `26.33.02`
(`fe2700e6b365763cc7dead36acb4e8b272bc99ac0eac46c6649acbdb01962796`)
and FYLO `26.33.02`
(`2c78f91130863ad797cf7a6062aa1245fa638857fee63b8671fa9e2748974fa3`).
All 14 identity black-box cases pass. This resolves the prior release-version
mismatch while retaining exact digest, release identity, dependency, protocol,
and operation checks. The same qualification run passed all 148 FYLO-backed
database tests and all 327 HTTP route tests.

Measured on macOS arm64, 2026-07-27, against SESAME `26.30.07`
(`c306e0937c89883c8a7209f0c37c849e05e6bffb84977b746c8cf4a8bd4da9c5`) and FYLO
`26.30.06` (`ae39a2b66ea9771766f3f3d6b3d0d1b01e1b3842a45aa0389535109b91bdee50`),
both verified against their published `SHA256SUMS`.

Gate: `bun run verify:phase3`, with `SESAME_TEST_BINARY` and `FYLO_TEST_BINARY`
pointing at the locked `26.33.02` artifacts.

## Released-engine qualification

14 of 14 passing, each against the real engine over the real protocol.

| Test | What it proves |
| --- | --- |
| `TestPhase3LoginIssuesVerifiableTokens` | A complete login issues an access, refresh, and ID token; the access token verifies against the engine's published keys and its claims agree with the issued session. |
| `TestPhase3AuthorizationIsDefaultDeny` | A mailbox owner reaches its own mailbox and nothing else: not a sibling mailbox in the same domain, not a mailbox in another domain, not domain administration. |
| `TestPhase3DomainRoleGrantsWholeDomain` | A member of a domain's admin group reaches every mailbox in that domain for read, write, and send, and no mailbox or configuration in another. |
| `TestPhase3OneIdentityAdministersSeveralDomains` | One mailbox joins two admin groups and one viewer group, signs in **once**, and holds exactly the authority each membership carries — including in a domain it has no mailbox in, and not in a domain it never joined. |
| `TestPhase3DomainAuthorityIsRevokedPerDomain` | Leaving one domain's group removes that domain's authority and leaves every other membership intact. |
| `TestPhase3DecisionNamesTheDomainItAuthorized` | A decision reports the domain of the resource it authorized, not the caller's own, so storage ownership follows the domain being acted in. |
| `TestPhase3RevokedSessionFailsClosedImmediately` | After revocation the token still verifies cryptographically but authentication, authorization, and refresh all fail — immediately, not at token expiry. |
| `TestPhase3SuspendedPrincipalFailsClosed` | A suspended mailbox stops authenticating on its existing token. |
| `TestPhase3RefreshRotatesAndDetectsReuse` | A refresh rotates the token and keeps the session; replaying the consumed token is refused and ends the family, so the replacement stops working too. |
| `TestPhase3MalformedAndForgedTokensFailClosed` | Nine rejected inputs: empty, non-token, two-segment, empty signature, truncated payload, reversed signature, `alg: none`, tampered tenant claim, and an ID token presented as an access token. |
| `TestPhase3PolicyRequiresPasswordWhenConfigured` | Under `password-and-passkey` the server reports both required factors, refuses a recovery code offered before any primary factor, refuses completion at password assurance, and lets the same transaction finish once the second factor lands. |
| `TestPhase3CredentialChangesRequireStepUp` | Recovery codes cannot be reissued without a satisfied step-up, and a consumed step-up cannot authorize a second change. |
| `TestPhase3BridgeOperationsRejectUnknownAndMalformed` | An unknown operation, a raw engine operation, an unknown request field, and a missing required field are each refused. |
| `TestPhase3OperationsAreAllPrefixed` | Every published operation is inside the `auth.` namespace. |

## The same contract at the persistence and route layers

The 2026-08-22 requalification has **148 passing database cases** and **327
passing route cases**, with no failures. `bun run verify:phase3` runs both
suites after the released-engine black-box cases.

Every suite starts the real Go owner, the released engine, and the released
FYLO, provisions a mailbox through the boundary, and signs in with the
credentials it is handed. Nothing in the harness can mint a token.

The legacy `tests/api` transport harness still starts through Tachyon. It is
therefore retained as a separate frontend/server migration gate rather than
being allowed to obscure FYLO/SESAME release qualification.

## Container lifecycle

The original 2026-07-27 production-image lifecycle proof used the then-current
released pair and a fresh bind-mounted volume. The current Dockerfile and
runtime lock pin FYLO and SESAME `26.33.02`; today’s qualification exercised
that pair outside the container. The historical lifecycle observations were:

| Check | Result |
| --- | --- |
| `caduceus auth init` on an empty volume | Created the deployment and its key boundary |
| Startup qualification | `fylo_version 26.30.06`, `sesame_version 26.30.07`, 28 auth operations, 6 qualification checks |
| `GET /health` | `200` |
| FYLO processes | **2**, each `--exclusive-root`, on `/data/caduceus` and `/data/sesame/fylo-root` |
| SESAME processes | **1**, `exec --loop --deployment /data/sesame` |
| Owner processes | **1**, `caduceus-go fylo-bridge` |
| `docker stop` | Bridge and engine stopped in order; exit code **0** |

One writer per root, two roots, one owner — the Phase 3 topology, observed in
the container rather than asserted about it.

Two image defects were found and fixed doing this:

- **The data roots were not created on a fresh volume.** The image pre-creates
  them, but a bind mount over `/data` hides everything the image put there, and
  that is a normal way to run this. The entrypoint now creates them; the Go
  owner still refuses one it cannot write to.
- **`/home/bun` was root-owned under the classic Docker builder.** BuildKit
  chowns implicitly created `COPY` parents and the classic builder does not, so
  the frontend bundler could not write its cache. The home directory is now
  prepared explicitly in the builder stage, making the image identical either
  way.

## Static gate

In the same run: `gofmt` clean over `internal`, `cmd`, `test`; `go vet ./...`
clean; `go test ./internal/...` passing; `bun run typecheck` and the strict
configuration both at zero errors.

The gate also refuses to pass unless the runtime lock pins the release, the
commit, the Go client pseudo-version, all five platform digests, and the
separate-FYLO-root fact; `go.mod` matches the pinned client; the deployment is
one tenant; domain authority goes through `GroupMemberAdd`, `GroupMemberRemove`,
and `GrantCreateForGroup`; the harness obtains tokens from a real login; and no
retired credential concept survives anywhere in `server`, `client`, `internal`,
`cmd`, or `scripts`.

## SMTP submission is refused

SMTP AUTH carries exactly one secret, and since this phase a password alone
cannot produce a session: every login must reach multi-factor assurance.
Submission previously called `/auth/password/verify`, which no longer exists.

It now refuses every login with an accurate reason rather than weakening the
policy for one protocol. The answer is an application password bound to a
SESAME workload principal holding only `mailbox:send` for one mailbox —
revocable on its own, without touching the human's credentials — and that is
Phase 6 work, where submission is owned. `MAIL_SUBMISSION_ENABLED` is off by
default and submission is not a supported feature before Phase 6, so this is a
documented gap rather than a regression in shipped behaviour.

## Four defects found and fixed during qualification

**A rejected completion erased proven factors.** `LoginComplete` cleared the
transaction's factor ledger before checking policy, so a caller who tried to
finish a login one factor early could never finish it at all — the password
they had already proven was gone. The ledger is now cleared only once the
transaction is actually consumed.

**An ID token was accepted as an access token.** Both are ES256, signed by the
same key, for the same audience, carrying the same session and tenant, so every
check passed. They are now distinguished by claim set — an access token always
carries `scope`, an ID token carries `acr` and no `scope` — and the difference
is checked rather than assumed.

**The settings view still called nine deleted endpoints.** Passkey, TOTP,
password, and recovery-code management all pointed at routes that no longer
exist, and the code compiled and typechecked cleanly because a route path is
just a string. Every one now targets the credential surface and goes through a
step-up prompt, because a credential change needs a factor proven again. The
gate gained a check for retired *route paths*, not just retired identifiers,
which is what would have caught this — and did immediately catch a second
live caller in the SMTP submission server.

**Guards named the wrong authorization target.** The mechanical migration off
role claims left several routes authorizing a domain read from `request.auth`,
which the guard itself populates and which is therefore always undefined. Each
route now names where its domain actually comes from, and a target the request
supplies is only used when it is well formed; otherwise the guard falls back to
the caller's own least authority and the handler rejects the malformed request
in its own words.

## Removed

`server/features/auth/` in full: the HS256 signer and verifier, the WebAuthn
service, the TOTP service, the password service, the authentication policy
module, the credential and MFA repositories — about 1,150 lines. With them:
`JWT_SECRET`, `AUTH_PASSWORD_PEPPER`,
`CADUCEUS_AUTH_STEP_UP_MAX_AGE_SECONDS`, the
the legacy `server/routes/auth/{password,mfa,recovery,totp,webauthn}` trees and
`server/routes/mfa` trees, `POST /test/seed/device` and the forged passkey
record it wrote, the MFA device rewrite in `scripts/migrate-domain.mjs`, and the
four suites that tested reimplementations of what the engine now owns.

## Limits

- **SESAME is a developer preview.** No release has passed a 72-hour native
  gate or an independent security review. CADUCEUS pins one qualified release by
  digest and fails closed on anything else. Production support is a Phase 8
  claim and this dependency is part of what must be qualified there.
- **The Playwright matrix is not green.** Three specs in
  `tests/e2e/login.test.mjs` fail on a freshly bundled client: clicking "Use a
  recovery code" leaves the form on the start step, so the recovery field never
  appears. The component itself is not at fault — imported directly,
  `useRecoveryCode()` moves `$step` to `recovery` and every handler the
  template binds exists — so the fault is in the browser render or event layer,
  and diagnosing it needs the page console.

  Two things were fixed getting that far, and both were real:

  - the Playwright `webServer` budget and the API server's own Tachyon wait were
    both 120s, which is no longer enough now that the API server boots the Go
    owner, the engine, and FYLO before Tachyon. Both are 300s.
  - the run that reported 26 passed / 6 failed was serving a **stale bundle**
    that predated the client rewrite, so its result described the old client.
    `bun run test:e2e` bundles first; invoking Playwright directly does not.

  Note `tachyon-26-28-breaking-changes`, which records a reconciler bug with
  slotless custom elements. `w-btn` is one, and the failure has that shape.
  Worth ruling in or out first.
- **Application passwords do not exist**, so SMTP submission is refused; see
  above.
- **Passkey registration and assertion are not covered end to end.** A headless
  browser has no platform authenticator, and a forged credential record can no
  longer exist, so the suites drive the recovery-code factor instead. The
  passkey path is exercised only through the engine's own tests.
