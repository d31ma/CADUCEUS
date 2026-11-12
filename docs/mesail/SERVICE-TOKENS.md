# Calling another Mesail service

A Yon app that calls another Mesail service (for example `mail-in` asking
`directory`) sends a `Service` access token. Since ADR 0005 that token is a
Mesail access token issued by `identity` (`POST
/api/v1/identity/service-token`), not a Keycloak token. The browser is not
involved, and Keycloak is not called.

## Settings

In the calling app's `server/application.json`:

```json
"oidc": {
  "issuer": "http://127.0.0.1:8101/api/v1/identity",
  "jwks": "http://127.0.0.1:8101/api/v1/identity/jwks",
  "audience": "mesail-api",
  "servicetokens": "http://127.0.0.1:8101/api/v1/identity/service-token"
},
"outgoing": { "hosts": ["127.0.0.1"] }
```

- `oidc.issuer` and `oidc.jwks` are the same in every app: the one Mesail
  issuer and `identity`'s key set. There is no `{realm}` placeholder any
  more.
- `oidc.servicetokens` is `identity`'s service token route. The key is all
  lowercase: TACHYON refuses an upper-case letter in an `application.json`
  key (TY2010).
- `outgoing.hosts` must list the `identity` host and the host of the
  service you call.

In the calling app's `.env` (never committed):

```text
YON_SERVICE_CLIENT_SECRET=<the mesail-service secret>
```

`identity` holds the same value (and, during a rotation,
`YON_SERVICE_CLIENT_PREVIOUS`; not `..._SECRET_PREVIOUS`, which `ty`
refuses because `service.client.secret` would be both a value and a
folder). Rotating it is an operator step: set
the new value as current and the old as previous in `identity`, then in
every caller's `.env`, then remove the previous value.

## Code

Call it from a controller, the identity provider or a `@Service` (slices
R1 and C2 showed under `ty start` 26.46.02 that a `@Service` that imports a
kit package is injected).

```go
authorization, err := oidc.ServiceTokensFactory{}.Configured().Authorization(domainId)
request.Header.Set("Authorization", authorization) // "Bearer <token>"
```

- `Authorization(domainId)` returns `Bearer <token>` whose `domain` claim
  is `domainId`. For install-wide calls pass `oidc.InstallWide{}.DomainId()` (`*`).
- Without the settings it returns `oidc.NotConfigured`. When `identity`
  refuses the client, it returns `oidc.SignInRefused`. When `identity`
  cannot answer, it returns `oidc.KeycloakUnavailable` (the name stays).
- The mode follows the settings: with `oidc.servicetokens` it asks
  `identity`; without it (the old per-realm settings, until slice L5) it
  asks Keycloak's `oidc.keycloak` with client credentials.
- Tokens are cached in `TMPDIR` (folder `mesail-oidc/tokens`, mode 0700,
  files mode 0600) until 10 seconds before they expire, so a warm handler
  asks `identity` about once a minute per domain.
- Use `kit/transport` `SingleUseTransport` or `http.DefaultTransport` for
  the call, so the egress guard of TACHYON sees it.

## Which domain

- Ask for the domain the call is about. The receiving service's
  `caller.DomainId` is then that domain, and the tenant rule holds: a
  receiver answers `404` to a `Service` call whose `DomainId` is not the
  domain in the path, body or event envelope.
- Install-wide calls (not about one hosted domain, for example asking
  whether a domain is hosted at all, or the scheduled jobs) use
  `oidc.InstallWide`. Only routes documented as install-wide accept a
  token whose domain is `*`.
- See also `docs/mesail/EVENTS.md` section 4.

## The receiving side

The route has `// @Allow(Service)`, and the app's `TokenIdentityProvider`
maps the token role `mesail-service` to `Service`. A service token has
`azp` `mesail-service`, which the token check accepts.

## Tests

Use `oidc.SigningRealm{}.StartedMesail()` to make a Mesail token without
`identity` (its settings: issuer
`https://mail.mesail.test/api/v1/identity`, `jwks` and `servicetokens`
under it; `ApplicationJson()` and `Publish(now)` as for `Started(name)`,
which stays the per-realm test issuer until slice L5):

```go
realm, _ := oidc.SigningRealm{}.StartedMesail()
token, _ := realm.Token(oidc.TokenRequest{Subject: "service", Domain: "example.com",
    RealmRoles: []string{"mesail-service"}, Audience: "mesail-api",
    AuthorizedParty: "mesail-service", ExpiresAt: time.Now().Unix() + 60})
```

To test the calling side, point `oidc.servicetokens` at an `httptest`
server that answers `POST` with `{"accessToken": "...", "expiresIn": 60}`.
