# Phase 3 — Authentication and authorization contract

CADUCEUS makes no authentication decision. One SESAME engine, owned by the Go
runtime, decides everything; this document is the contract around it.

Release identity, pinned in `runtime.lock.json`:

| Artifact | Value |
| --- | --- |
| SESAME release | `26.33.02` |
| Release commit | `e8f36b6cf946a6979b2e2aa6df604e4ef6d8f1cb` |
| Go client | `github.com/d31ma/sesame v0.0.0-20260811145757-e8f36b6cf946` |
| Machine protocol | `1` |
| linux-x64 | `d0f2ca94937b73b1cfcf9f89a48783cf0a1cf317cba8a83753be24dea2ef07bc` |
| linux-arm64 | `cd6c0e5ce0e353690bc9bfc81a30e05ebff11cab4b1f806a83995bc8d12d3ceb` |
| macos-arm64 | `fe2700e6b365763cc7dead36acb4e8b272bc99ac0eac46c6649acbdb01962796` |

The `v26.33.02` tag's major version is not valid for the module path, so the Go
client resolves the same tagged commit as the pinned pseudo-version.

## Process topology

```text
caduceus fylo-bridge (Go, one process)
  ├── FYLO 26.33.02 ── exclusive root ── mail documents and attachment buckets
  ├── SESAME 26.33.02 ── its own FYLO 26.33.02 root ── security ledger
  └── loopback bridge ── HMAC, replay-resistant, one frame per connection
        └── Bun runtime ── HTTP routes ── browser
```

One FYLO process owns one root. The identity ledger and the mail store are
separate writers over separate data, so the single-writer invariant holds for
both. The bridge is the only path from the legacy runtime to either.

SESAME and the mail-store owner intentionally use separate FYLO processes and
roots, but both now pin the same `26.33.02` release and dependency pair.

## Startup qualification

The runtime refuses to open its listener unless all of the following hold:

- the SESAME binary's SHA-256 matches a digest pinned in `runtime.lock.json`;
- the deployment directory exists and contains `config.json`,
  `keys/signing.key`, and `keys/secrets.key`;
- the engine reports the pinned release version;
- the engine speaks machine protocol `1`;
- the engine routes all 36 operations CADUCEUS calls;
- the engine reports storage-ready.

The deployment is never created implicitly. Minting a new signing key on a
volume whose keys failed to mount would end every session with no explanation,
so `caduceus auth init` is an explicit act and refuses an existing deployment.

## Authentication ladder

A login is one transaction. Each step supplies a factor and reads back the
transaction's state; the server reports which factors this deployment requires
and whether they have been satisfied. The client decides nothing.

| Policy | Required factors | Notes |
| --- | --- | --- |
| `passkey-first` (default) | passkey | A user-verified passkey is phishing-resistant and reaches MFA assurance alone. |
| `password-and-passkey` | password, passkey | Both must actually be verified. |

Both policies require MFA assurance before a session is issued. The engine will
complete a transaction at password assurance; CADUCEUS will not, and revokes
any session it is handed below policy.

A recovery code is a second factor: the engine closes a transaction offered one
before any primary factor. This is what a newly provisioned mailbox holds, so
its first login is password plus recovery code, after which the owner enrols a
passkey through the ordinary step-up path.

A completion attempt that the policy rejects is not terminal. The transaction
stays live and the factors already proven are kept, so a caller can supply what
is missing and finish the same login.

## Tokens

`authn.complete` issues a session. The session is exchanged for a token set
through the authorization-code flow with PKCE, run entirely in-process:
CADUCEUS is both the authorization host and its own confidential first-party
client, so no browser redirect is involved.

The access token is ES256, audience-bound to the registered client, and carries
`sub` (principal), `tenant_id`, `sid` (session), `scope`, and the standard time
claims. Its lifetime is the engine's — five minutes — and `offline_access`
yields a refresh token that rotates on every use with reuse detection.

Verification is two checks for two different facts:

1. **Signature, locally.** Proves the token was minted by this deployment and
   not altered. Rejects a forged signature, an algorithm downgrade, a claim
   edit, an unknown key, and an expired or not-yet-valid token without touching
   the engine.
2. **Liveness, at the engine.** Proves the grant still stands. Revocation
   cannot be written into an already-signed token, so a revoked session or a
   suspended principal fails on the next request rather than when the token
   would have expired.

An ID token is signed by the same key for the same audience with the same
session and tenant. It is refused where an access token is required: an ID
token carries `acr` and no `scope`, an access token always carries `scope`, and
the difference is checked rather than assumed.

## Tenancy and multi-domain administration

One deployment is one SESAME tenant, named `caduceus`. Every mailbox in every
mail domain is a principal in it.

A mail domain is not a tenant, because SESAME group membership requires the
principal and the group to share one. Domain-per-tenant would make a person who
administers two domains two identities with two logins, which is exactly the
thing groups exist to avoid.

So a domain is a resource, and authority over it is a group membership:

| Object | Name | Purpose |
| --- | --- | --- |
| Role | `domain-admin-<label>` | Every permission over one domain |
| Role | `domain-viewer-<label>` | Read-only over one domain |
| Group | `domain-admins-<label>` | Holds the admin role; membership grants it |
| Group | `domain-viewers-<label>` | Holds the viewer role |
| Role | `mailbox-<digest>` | Read, write, and send for exactly one mailbox |

`<label>` is `slug(domain)-<8 hex of sha256(domain)>`, bounded to fit the 63-byte
name limit. The mailbox role is a digest because a local part may be 64 bytes on
its own; role names are not the audit surface, resources are, and
`mailbox:corp.com:chi` stays legible everywhere it matters.

The role is granted to the group once, when the domain is provisioned. After
that, giving or taking away domain authority is a membership change and touches
no grants:

```bash
caduceus auth role   --address ops@corp.com --domain partner.com --role domain-admin
caduceus auth unrole --address ops@corp.com --domain partner.com --role domain-admin
```

One mailbox may be a member of as many domain groups as it administers and
still signs in once. Leaving one domain's group takes that domain's authority
and no other's.

What this gives up is SESAME's tenant boundary between mail domains. What
enforces separation instead is unchanged: every decision is default-deny over a
domain-qualified resource, and FYLO's per-domain UID/GID ownership sits below
any caller-controlled filter. Storage ownership follows the domain the decision
authorized, not the caller's own address, so a cross-domain administrator writes
into the domain it is acting in.

## Authorization vocabulary

Actions and resources are colon-joined lowercase segments. Only a role
permission may end in a wildcard.

| Action | Meaning |
| --- | --- |
| `mailbox:read` | Read messages, folders, threads, search, attachments |
| `mailbox:write` | Mutate message state, drafts, rules, templates, signatures |
| `mailbox:send` | Submit and schedule outbound mail |
| `domain:read` | Read one domain's configuration |
| `domain:admin` | Change one domain's configuration, mailboxes, and routes |

| Resource | Held by |
| --- | --- |
| `mailbox:<domain>:<local>` | The mailbox owner |
| `mailbox:<domain>:*` | `domain-admin`, and `mailbox:read` for `domain-viewer` |
| `domain:<domain>` | `domain-admin`, read-only for `domain-viewer` |

A mailbox local part is restricted to letters, digits, and `-_.` so a resource
reads as itself in a decision trace and in the security ledger. Plus-addressing
is a delivery-time concept applied to a mailbox that already exists and is
unaffected.

Tenant names are a bounded slug, so a mail domain never travels as one. The
mapping is `slug(domain)-<8 hex of sha256(domain)>`, which keeps distinct
domains that share a slug apart: `a-b.com` and `a.b-com` both slug to `a-b-com`.

## Credential changes

Changing a credential requires a freshly satisfied authentication transaction,
not a session established at some point in the past, and not a timestamp. The
boundary completes the step-up transaction, compares its principal with the
access token's, and revokes the proof session immediately — the engine will not
say whose transaction it is, but it will issue a session, and that session
names its principal.

A step-up is single use. Removing a passkey additionally requires that the
credential belongs to the caller, and reports the same way for a credential
that belongs to someone else as for one that does not exist.

## Operator commands

```bash
caduceus auth init                                         # create the key boundary
caduceus auth domain   --domain example.com                # tenant and domain roles
caduceus auth mailbox  --address user@example.com          # principal, role, grant
caduceus auth role     --address user@example.com --domain example.com --role domain-admin
caduceus auth unrole   --address user@example.com --domain example.com --role domain-admin
caduceus auth credentials --address user@example.com --password '...'
caduceus auth suspend  --address user@example.com
caduceus auth status
```

`auth credentials` prints a recovery code set once. It is what a new mailbox
owner uses to reach MFA assurance on a first login.

Provisioning runs while the service is stopped, or through the bridge: the
engine's FYLO root has one writer, and a second one is refused.

The deployment directory also holds `caduceus-registry.json` (mode 0600): the
per-tenant OIDC client secrets and the identifiers of the roles and groups this
deployment created. SESAME roles and groups are immutable and tenant-unique and cannot be read
back by name, so losing this file does not lose them — it loses the ability to
grant or join them again. Back it up with the keys.

## Configuration

| Key | Meaning |
| --- | --- |
| `SESAME_BINARY` | The qualified engine executable |
| `SESAME_DEPLOYMENT` | Deployment directory: keys, config, engine FYLO root |
| `SESAME_ISSUER` | `https` base URL tokens are issued under |
| `SESAME_REDIRECT_URI` | Registered redirect URI; defaults to `PUBLIC_URL/auth/callback` |
| `SESAME_SESSION_LIFETIME` | Session bound, `1m`–`720h`, default `12h` |
| `SESAME_REQUEST_TIMEOUT` | Per-operation deadline, `100ms`–`2m`, default `10s` |
| `CADUCEUS_AUTH_POLICY` | `passkey-first` or `password-and-passkey` |

Retired: `JWT_SECRET`, `AUTH_PASSWORD_PEPPER`,
`CADUCEUS_AUTH_STEP_UP_MAX_AGE_SECONDS`.

## HTTP surface

| Route | Purpose |
| --- | --- |
| `POST /auth/login/begin` | Open a transaction for an address |
| `POST /auth/login/password` | Supply the password factor |
| `POST /auth/login/passkey/options` | Challenge for the in-flight transaction |
| `POST /auth/login/passkey` | Supply a WebAuthn assertion |
| `POST /auth/login/totp` | Supply an authenticator code |
| `POST /auth/login/recovery` | Spend a single-use recovery code |
| `POST /auth/login/complete` | Issue the session and its token set |
| `POST /auth/token/refresh` | Rotate a refresh token |
| `GET`/`DELETE /auth/session` | Read or end the caller's own session |
| `POST /auth/logout` | End the session an ID token speaks for |
| `POST /auth/credentials/passkey/begin`/`finish` | Enrol a passkey |
| `GET`/`DELETE /auth/credentials/passkey` | List or remove own passkeys |
| `POST /auth/credentials/totp/enroll`/`activate` | Enrol and activate TOTP |
| `POST /auth/credentials/recovery` | Replace the recovery code set |
| `PUT /auth/credentials/password` | Set the caller's password |
| `GET /auth/jwks` | Public signing keys |

Every one of these is a thin adapter. It validates the shape of the request,
carries it to the boundary, and reports the answer.
