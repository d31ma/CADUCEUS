# ADR 0005: One login per person, the domain as membership data, Mesail's own access token

- Status: Accepted
- Date: 2026-11-11
- Amends: [ADR 0004](0004-mesail-go-serverless-keycloak.md) section 2
  (one Keycloak realm per domain) and section 7 (Keycloak tokens in the
  browser and between services). Everything else in ADR 0004 stays.
- Spec: [SPEC.md](../mesail/SPEC.md) AC-ID-1 to AC-ID-14. Contracts:
  [CONTRACTS.md](../mesail/CONTRACTS.md) "identity" and "directory".
  Slices: [SLICES.md](../mesail/SLICES.md) "Wave 3: one login".

## Context

The owner's requirement (2026-11-11): "keep the login simple for the
user. they should have 1 login instead of remembering different logins for
different domains. they should use their email prefix (before the `@`
character) as a username -- as the same prefix should be used in all
domains."

The owner's answers to the follow-up questions:

1. A person belongs only to the domains an admin adds them to. The prefix
   is reserved on the whole install: no one else may take `ana@` in any
   domain. Ana gets an address only in the domains an admin adds her to,
   and after sign-in she switches between her domains.
2. The sign-in form accepts the prefix or the full address: `ana` and
   `ana@example.com` sign in to the same account.

Rules that do not change: chat never crosses a domain; email does; only
the backend talks to Keycloak; the browser holds only Mesail's own token;
Keycloak runs on SQLite in one container.

ADR 0004 made each domain one Keycloak realm with the same name. The token
issuer named the domain, so every service knew the caller's domain without
a lookup. A person with addresses in two domains had two accounts. That
model cannot give one login: a Keycloak user lives in exactly one realm,
and a username is unique only inside its realm.

## Options

**A. Keep one realm per domain and link the accounts.** Keycloak would
keep one user per domain and Mesail would sync passwords and second
factors between them. Rejected: credentials in several places drift, a
password change is no longer one change, and brute-force detection counts
per realm, so an attacker gets one budget per domain.

**B. One realm, Keycloak tokens name the active domain.** One realm
`mesail`, and a Keycloak provider adds a `domain` claim from a session
note that a custom endpoint changes on a domain switch. Membership would
live in Keycloak groups. Rejected: the switch needs a new Java endpoint and
a new mapper, the domain admin role would need group rules in Keycloak,
membership would live in Keycloak while the address table lives in
`directory`, and a service token would still need a way to name a domain.

**C. One realm for credentials; membership is Mesail data; `identity`
issues Mesail's own access token that names the active domain.** Chosen.

## Decision

### 1. One Keycloak realm for every person
- Keycloak has one realm, `mesail`. It holds every person: members of any
  domain and Operators. Keycloak owns only credentials and sessions:
  password, 2-step code (TOTP), brute-force detection, session lifetime,
  and the realm role `mesail-operator`.
- The Keycloak **username is the person's prefix** (the local part of all
  their addresses), lowercase. It never changes (`editUsernameAllowed`
  false). Email sign-in is off in Keycloak (`loginWithEmailAllowed` false);
  `identity` turns a full address into the username itself.
- The realms `mesail-operators` and one realm per domain are removed, and
  so is the realm template. Adding a domain no longer touches Keycloak.
- Keycloak clients in the realm: `mesail-identity` (confidential, direct
  access grants, conditional OTP), `mesail-account` (service account with
  `manage-users` and `view-users`, used by `identity` for the person's own
  password and 2-step code), `mesail-directory` (service account with
  `manage-users`, `view-users` and `query-users`, used by `directory` to
  create, disable and delete people and to reset their credentials). The
  client `mesail-service` is removed from Keycloak (section 4).

### 2. A person, members and the reserved prefix are `directory` data
- A **person** is one Keycloak user. `directory` keeps the install-wide
  collection `people` (`personId` = the Keycloak `sub`, `username`,
  `displayName`, `domainIds[]`, `adminDomainIds[]`, `defaultDomainId`,
  `state`). `domainIds[]` is the list of domains the person may sign in to.
- A **member** is a person in one domain (`members-<domainKey>`, as now).
  `memberId` is the `personId`, the same in every domain, so a member id is
  still the token `sub`. Each domain's data stays keyed by the domain, so
  the same id in two domains never joins their data.
- **A prefix is reserved on the whole install.** `directory` keeps the
  install-wide collection `local-parts`: one document per local part used
  by a person, an alias or a channel address. A person's username can never
  be an alias or a channel address in any domain, and a new person can
  never take a local part that an alias or channel already uses in any
  domain. Keycloak's unique username is the last guard.
- A member's primary address in a domain is always `<username>@<domainId>`.
  Aliases stay per domain.
- Only a DomainAdmin of the domain (or an Operator) adds or removes a
  member. Adding a new prefix creates the person in Keycloak; adding a
  prefix that already belongs to a person adds that person to the domain.
- The DomainAdmin role is membership data (`adminDomainIds[]`), not a
  Keycloak role. A DomainAdmin cannot reach Keycloak and cannot give
  anyone the Operator role.
- **Credential resets** (a forgotten password, a lost authenticator, turning
  a person off) change the person's one account, so they affect every
  domain of that person. An Operator may do them. A DomainAdmin may do them
  only for a person whose every domain the DomainAdmin also administers.
  Otherwise one domain's admin could take over an account that also reads
  another domain.

### 3. `identity` issues Mesail's access token; the token names the active domain
- `identity` signs in to the realm `mesail` server to server (direct
  access grants, as before) and answers with **its own access token**: a
  JWT signed with ES256 under `identity`'s key (secret store,
  `YON_IDENTITY_SIGNING_KEY`, with `YON_IDENTITY_SIGNING_KEY_PREVIOUS` kept
  during a rotation). Keycloak's own tokens never leave `identity`.
- Claims: `iss` (the configured Mesail issuer), `aud` `mesail-api`, `sub`
  (the person id, or `service` for a service token), `azp` (`mesail-web` or
  `mesail-service`), **`domain`** (the active domain id, or `*`),
  `realm_access.roles` (`mesail-member`, `mesail-admin` when the person
  administers the active domain, `mesail-operator` from Keycloak, or
  `mesail-service`), `name`, `preferred_username`, `email`
  (`<username>@<domain>`), `sid` (the Keycloak session id), `iat`, `nbf`,
  `exp` (60 seconds), `jti`. The claim names stay those of ADR 0004, so
  `kit/oidc` `RoleMap` does not change.
- `identity` publishes its public keys at `GET /api/v1/identity/jwks`.
  Every service checks tokens against that key set and one exact issuer.
  `Caller.DomainId` comes from the `domain` claim instead of the realm in
  the issuer. Nothing else in the services changes: no route takes the
  caller's domain from the request.
- `*` is the **install-wide** domain. Only an Operator session with no
  domain membership, and an install-wide service token, carry it. A store
  method refuses `*` like any domain that is not hosted, and only routes
  documented as install-wide accept it. Operator routes check the role
  `Operator`, which only Keycloak's realm role gives; they never use the
  active domain as a tenant.
- **Sign-in** takes a prefix or a full address. `identity` lowercases and
  trims it and splits it at `@`. The prefix is the Keycloak username. The
  domain part, when given, chooses the first active domain; without it,
  the person's `defaultDomainId` is used. `identity` reads the person's
  domains from `directory` (`GET /api/v1/directory/people/{personId}`,
  install-wide `Service`).
- **Domain switch** needs no password: `identity` refreshes the Keycloak
  session from the `sessionKey`, checks that the person is still a member
  of the wanted domain, and issues a token for that domain and a new
  `sessionKey`. The `sessionKey` now seals the refresh token and the active
  domain. Each browser tab keeps its own `@SessionState`, so two tabs may
  show two domains of the same person.
- **Refresh** checks the membership again, so removing a person from a
  domain cuts their access to it within 60 seconds, like disabling them in
  Keycloak.

### 4. Service tokens come from `identity`
- A service asks `identity` for a token: `POST
  /api/v1/identity/service-token` with the shared service client id and
  secret and the target `domainId` (or `*`). `identity` compares the secret
  in constant time (current and previous value, for rotation) and answers
  a Mesail token with `azp` `mesail-service`, the role `mesail-service`
  and that `domain`.
- `kit/oidc` `ServiceTokens.Authorization(domainId)` keeps its signature
  and its per-domain cache. `oidc.InstallWide` (`*`) replaces the realm
  name `mesail-operators` in calls that are about the whole install.
- Why not Keycloak client credentials: one realm cannot name a domain in a
  client-credentials token without a new Keycloak provider. The secret had
  the same power before (every service could get a `Service` token for any
  domain), so `identity` holding it adds no new power, and a Keycloak
  outage no longer stops calls between services.

## Consequences

Good:
- One login, one password, one 2-step code and one brute-force budget per
  person. A domain switch is one call with no password.
- Membership, admin rights and addresses live in one place (`directory`),
  so the address table and the right to sign in to a domain can never
  disagree.
- Adding a domain is a plain `directory` write; no Keycloak admin call and
  no realm template.
- Services change only their token settings (issuer and key set URL);
  their code keeps using `Caller.DomainId`.

Costs, accepted:
- `identity` becomes a token issuer: it holds a signing key, and every
  service depends on its key set. A stolen signing key lets an attacker
  act in any domain, as a stolen Keycloak realm key did before; the key
  lives only in the secret store, and rotation keeps two keys published.
- Every sign-in, refresh and switch reads the person from `directory`
  (about one call per active tab per minute).
- The prefix namespace is shared by every domain on the install. This fits
  an install whose domains belong to one organization or to people who
  trust each other. Two unrelated organizations on one install cannot both
  have an `info` person; they can still both have an `info` alias.
- A person's display name is the same in every domain.
- A DomainAdmin can add an existing person to their domain without that
  person's consent, and learns that the prefix exists (the owner's answer
  1, confirmed as SPEC Q4 on 2026-11-11).
- A deleted person's prefix is never reused (SPEC Q5), so the namespace
  only grows.

## Migration

No production data exists. The dev Keycloak database and the dev users are
rebuilt from the new realm file. FYLO test roots keep working: member ids
change to the new person ids, so dev data that names old member ids is
reset with the stack (P1).

## Requests to dependencies

- TACHYON: none new. The same-origin credentials request of ADR 0004 still
  stands; with it the `sessionKey` moves to an `HttpOnly` cookie.
