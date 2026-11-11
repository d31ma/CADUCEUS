# Mesail

Mesail is one app that is both a team messenger and an email client and
server. It is the rewrite of CADUCEUS.

- Workspaces, channels, direct messages and threads, like Slack or Teams.
- One composer: a colleague with a Mesail account gets a chat message, any
  other address gets an email. You see and can change the choice for every
  recipient.
- Email from outside shows up as a conversation.
- Several mail domains in one install.
- Sign-in through Keycloak (OpenID Connect).

The frontend (Tac) and the backend (Yon) are Go, built with
[TACHYON](https://github.com/d31ma/TACHYON) 26.46.02. Data lives in
[FYLO](https://github.com/d31ma/FYLO) 26.45.07, one database per service.

## Status

Under construction. The plan and the contracts are in place; services are
built slice by slice.

| Document | What it says |
| --- | --- |
| [docs/mesail/SPEC.md](docs/mesail/SPEC.md) | user scenarios and acceptance criteria |
| [docs/adr/0004-mesail-go-serverless-keycloak.md](docs/adr/0004-mesail-go-serverless-keycloak.md) | the architecture decision |
| [docs/mesail/DOMAIN.md](docs/mesail/DOMAIN.md) | entities, FYLO collections, invariants, events |
| [docs/mesail/CONTRACTS.md](docs/mesail/CONTRACTS.md) | HTTP routes and models of every service |
| [docs/mesail/SLICES.md](docs/mesail/SLICES.md) | implementation slices and file ownership |
| [docs/THREAT_MODEL.md](docs/THREAT_MODEL.md) | threats and required controls (CADUCEUS; still applies) |

## Layout

```text
apps/<service>/server/   one Yon app per backend service (Go)
apps/web/client/         the messenger UI (Go Tac)
apps/web-admin/client/   the admin UI (Go Tac)
kit/                     shared plain Go packages, copied into apps by scripts/kit-sync.sh
tests/blackbox/          acceptance tests over HTTP and SMTP (build tag blackbox)
tools/crap/              CRAP score gate for plain Go packages
scripts/gates.sh         every local gate
```

## Gates

```sh
scripts/gates.sh                  # every app
scripts/gates.sh --app messaging  # one app plus the shared checks
```

`scripts/gates.sh` needs `ty` 26.46.02 (set `MESAIL_TY` to its path if it is
not the pinned copy), Go 1.27 or newer, and TinyGo with Binaryen for small
release WebAssembly.

The CADUCEUS code (JavaScript server and client, the Go monolith and
SESAME) was removed when Mesail started. It remains in git history at commit
`4ef82f72` (branch `go/rewrite`).

## License

See [LICENSE](LICENSE).
