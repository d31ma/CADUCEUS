# Mesail Keycloak realms

The realm templates live in `deploy/keycloak/realm/` (owned by the
`mesail-keycloak` session). This file lists what the identity service and
the token check in `kit/oidc` need from them.

## One realm per mail domain

- The realm name is the domain name in lowercase, for example `example.com`.
- Operators sign in to the realm `mesail-operators`, which is not a mail domain.
- Keycloak is reached only from the backend. The browser never talks to it.

## Clients

- `mesail-identity`: confidential, direct access grants only. The identity
  service signs people in with it. Every realm uses the same secret
  (`YON_IDENTITY_CLIENT_SECRET` in identity's `.env`).
- `mesail-service`: client credentials, for calls between services. Also in
  `mesail-operators`, for install-wide calls.
- `mesail-account`: client credentials only. Its service account has only
  the realm-management roles `manage-users` and `view-users`. identity uses
  it for the account settings (password, 2-step code). Shared secret
  `YON_IDENTITY_ACCOUNT_SECRET`.
- `mesail-api`: no sign-in. It exists only as the token audience.

## What the token check requires

| Claim | Rule |
| --- | --- |
| `iss` | Matches `oidc.issuer`, for example `http://127.0.0.1:8180/idp/realms/{realm}`. The `{realm}` part is the domain. |
| signature | RS256 or ES256, made by a key in the realm's key set (`oidc.jwks`) with the token's `kid`. |
| `aud` | Contains `mesail-api`. |
| `azp` | `mesail-identity` or `mesail-service`. |
| `typ` | `Bearer`. |
| `exp` | Not passed by more than 30 seconds. |
| `nbf` | If present, not more than 30 seconds in the future. |
| `sub` | Present. It is the member id. |

Mesail also reads `name`, `email` and `realm_access.roles`.

## Roles

| Keycloak realm role | Mesail role |
| --- | --- |
| `mesail-member` | `Member` |
| `mesail-admin` | `DomainAdmin` (own domain only) |
| `mesail-operator` | `Operator` |
| `mesail-service` | `Service` |

## Service settings

Every service, in `server/application.json`:

```json
{
  "oidc": {
    "issuer": "http://127.0.0.1:8180/idp/realms/{realm}",
    "jwks": "http://127.0.0.1:8180/idp/realms/{realm}/protocol/openid-connect/certs",
    "audience": "mesail-api"
  },
  "outgoing": { "hosts": ["127.0.0.1"] }
}
```

Without all three `oidc` keys a service refuses every token.

The identity service also needs:

- `oidc.keycloak` in `server/application.json`: the base of the token
  and logout endpoints, `http://127.0.0.1:8180/idp/realms/{realm}/protocol/openid-connect`.
- `oidc.admin` in `server/application.json`: the admin API base,
  `http://127.0.0.1:8180/idp/admin/realms/{realm}`.
- In its `.env` (never committed): `YON_IDENTITY_CLIENT_SECRET` (the
  `mesail-identity` secret), `YON_IDENTITY_ACCOUNT_SECRET` (the
  `mesail-account` secret) and `YON_IDENTITY_SESSION_KEY` (32 random bytes
  in standard base64, for example `openssl rand -base64 32`). The session
  key encrypts the `sessionKey` and the 2-step `setupKey` with AES-GCM.

## Admin API calls identity makes

All for the user whose `sub` is in the caller's token, never an id from the
request:

- `PUT  /admin/realms/<domain>/users/<sub>/reset-password`
- `GET  /admin/realms/<domain>/users/<sub>/sessions`, then
  `DELETE /admin/realms/<domain>/sessions/<id>` for every session except the
  caller's own
- `GET  /admin/realms/<domain>/users/<sub>/credentials`, then
  `DELETE /admin/realms/<domain>/users/<sub>/credentials/<id>` for type `otp`
- `PUT  /admin/realms/<domain>/mesail/users/<sub>/otp` with
  `{secret (base32), digits 6, period 30, algorithm HmacSHA1}`: stores the
  2-step credential. This is an endpoint of mesail-keycloak's provider
  (being settled), because the stock admin API cannot set an OTP secret.

## Rotating a shared secret

Set the new value in every realm (client `mesail-identity`, `mesail-account`
or `mesail-service`), then in every `.env` that holds it, then restart those
services.

Without these, the session routes answer `503 unavailable`.
