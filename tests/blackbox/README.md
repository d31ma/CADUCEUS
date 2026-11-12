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
    MESAIL_PASSWORD=... MESAIL_PASSWORD_OPERATOR=... \
    MESAIL_SERVICE_CLIENT_SECRET=... \
    MESAIL_TOKEN_ANA=... MESAIL_TOKEN_BO=... MESAIL_TOKEN_EVE=... \
    MESAIL_TOKEN_OPERATOR=... \
    go test -tags blackbox -v ./...

Fixtures they expect (ADR 0005: one login per person, one Keycloak realm
`mesail`, the prefix is the username in every domain). Email crosses
domains; chat never does.

- `ana`: DomainAdmin of `example.com` (her default domain) and a plain
  member of `example.org`. Token `MESAIL_TOKEN_ANA` is her `example.com`
  token.
- `bo`: member of `example.com` (`MESAIL_TOKEN_BO`).
- `eve`: member of `example.org` (`MESAIL_TOKEN_EVE`).
- `carol`: member of `partner.test` only.
- `operator`: Keycloak role `mesail-operator`, in no domain
  (`MESAIL_TOKEN_OPERATOR` is an install-wide session, domain `*`).
- `MESAIL_PASSWORD` is the password of the fixture people;
  `MESAIL_PASSWORD_<NAME>` overrides it for one of them (for example
  `MESAIL_PASSWORD_OPERATOR`). The sign-in tests sign in by themselves.
- `MESAIL_SERVICE_CLIENT_SECRET` is the `mesail-service` secret, for the
  service token tests.
- `MESAIL_SIGNIN_LOOP` (a Go duration, default `20s`) is how long
  `TestK2SignInStaysQuickWhileAPersonIsDeleted` signs in as `ana` in a loop
  after deleting a new person; every sign-in must answer 200 within 2
  seconds. Use `240s` or more to cover Keycloak's first session purge
  (about 180 seconds after Keycloak starts; slice K2).

The one-login tests (`SignIn_test.go`, `DomainSwitch_test.go`,
`Tokens_test.go`, `Members_test.go`, `People_test.go`; slice L0) create
new people with prefixes `bb<time>` and new domains `bb<time>.test`.
Prefixes are never reused (SPEC Q5), so these stay in the dev stack; reset
the stack to clear them. Wrong-password tests use new people, so the
fixtures are never locked by brute-force detection. The tests that wait
for a membership change allow 70 seconds.

The offline-first tests (`Sync_test.go`, `Queue_test.go`,
`KeptDevice_test.go`; slice O0, ADR 0006) send direct messages from ana to
bo, wait up to 15 seconds for the change log, and create new people for the
session tests, so the fixtures' own sessions are never revoked.

A missing variable skips the test.
