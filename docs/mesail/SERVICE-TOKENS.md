# Calling another Mesail service

A Yon app that calls another Mesail service (for example `mail-in` asking
`directory`) sends a `Service` access token. It gets that token from
Keycloak itself, with the client credentials of the client `mesail-service`.
The browser and the `identity` service are not involved.

## Settings

In the calling app's `server/application.json`:

```json
"oidc": {
  "issuer": "http://127.0.0.1:8180/idp/realms/{realm}",
  "jwks": "http://127.0.0.1:8180/idp/realms/{realm}/protocol/openid-connect/certs",
  "audience": "mesail-api",
  "keycloak": "http://127.0.0.1:8180/idp/realms/{realm}/protocol/openid-connect"
},
"outgoing": { "hosts": ["127.0.0.1"] }
```

- `oidc.keycloak` is the base of the token endpoint (`<base>/token`).
- `outgoing.hosts` must list the Keycloak host and the host of the service
  you call.

In the calling app's `.env` (never committed):

```text
YON_SERVICE_CLIENT_SECRET=<the mesail-service secret>
```

Every realm uses the same `mesail-service` secret (`MESAIL_SERVICE_SECRET`
in the Keycloak image). Rotating it is an operator step: change it in every
realm, then in every `.env`, then restart.

## Code

Call it from a controller or the identity provider (a `@Service` that
imports a kit package cannot be injected in TACHYON 26.46.02):

```go
authorization, err := oidc.ServiceTokensFactory{}.Configured().Authorization(domainId)
request.Header.Set("Authorization", authorization) // "Bearer <token>"
```

- `Authorization(realm)` returns `Bearer <token>` for that realm.
- Without the settings it returns `oidc.NotConfigured`. When Keycloak
  refuses the client, it returns `oidc.SignInRefused`. When Keycloak cannot
  answer, it returns `oidc.KeycloakUnavailable`.
- Tokens are cached in `TMPDIR` (folder `mesail-oidc/tokens`, mode 0700)
  until 10 seconds before they expire, so a warm handler asks Keycloak
  about once a minute per realm.
- Use `http.DefaultTransport` (a plain `http.Client{}`) for the call, so the
  egress guard of TACHYON sees it.

## Which realm

Ask for the token in the realm of the domain the call is about. Then the
receiving service's `caller.DomainId` is that domain, and the tenant rule
holds without exceptions: the receiver refuses a `Service` call whose
`DomainId` is not the domain in the path or event envelope.

For a call that is not about a hosted domain (for example asking whether a
domain is hosted at all), the architecture decides the realm; see
`CONTRACTS.md`. Until it does, `mesail-operators` is the proposal.

## The receiving side

Nothing changes. The route has `// @Allow(Service)`, and the app's
`TokenIdentityProvider` maps the realm role `mesail-service` to `Service`.
A service token has `azp` `mesail-service`, which the token check accepts.

## Tests

Use `oidc.SigningRealm` to make a `Service` token without Keycloak:

```go
token, _ := realm.Token(oidc.TokenRequest{Subject: "service-account", RealmRoles: []string{"mesail-service"},
    Audience: "mesail-api", AuthorizedParty: "mesail-service", ExpiresAt: time.Now().Unix() + 60})
```

To test the calling side, point `oidc.keycloak` at an `httptest` server that
answers `POST .../token` with `{"access_token": "...", "expires_in": 60}`.
