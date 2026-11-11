# Mesail blackbox tests

Throwaway acceptance tests written by the architect. They define the
contract from the outside (HTTP and SMTP) and name the SPEC criteria
(`AcCp4` means AC-CP-4). Delete a test only when the slice's own tests cover
the same behavior.

They are behind the `blackbox` build tag, so `go vet ./...` and
`go test ./...` skip them. The gate only compiles them:

    go vet -tags blackbox ./...

To run them against a running Mesail (all-in-one image or `ty preview` of
each app behind the router):

    MESAIL_BASE_URL=http://127.0.0.1:8080 \
    MESAIL_SMTP_ADDR=127.0.0.1:2525 \
    MESAIL_TOKEN_ANA=... MESAIL_TOKEN_BO=... MESAIL_TOKEN_EVE=... \
    MESAIL_TOKEN_OPERATOR=... \
    go test -tags blackbox -v ./...

Fixtures they expect. Each hosted mail domain is its own tenant, with its
own Keycloak realm. Email crosses domains; chat never does.

- Hosted domain `example.com`: `ana@example.com` (realm role
  `mesail-admin`, token `MESAIL_TOKEN_ANA`) and `bo@example.com` (realm
  role `mesail-member`, token `MESAIL_TOKEN_BO`).
- Hosted domain `example.org`, on the same Mesail: `eve@example.org`
  (realm role `mesail-member`, token `MESAIL_TOKEN_EVE`).
- Operator: realm `mesail-operators`, realm role `mesail-operator`, token
  `MESAIL_TOKEN_OPERATOR`.
- External address `carol@partner.test` (not hosted).

A missing variable skips the test.
