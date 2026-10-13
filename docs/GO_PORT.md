# Porting the CADUCEUS server to Go

The server is written in Go on Tachyon Yon, and the repository runs on it:
the container entry point, the mail services and the server are Go. The
JavaScript server it replaced is kept at `tests/support/legacy-server/` only as
the reference for behaviour and as the test suites' way to seed records (and
for the `admin:create` and `domain:migrate` operator scripts). Port it
faithfully: same routes, same request and response JSON, same FYLO documents,
same error messages and statuses. This is a rewrite of the language, not of
the product.

## Layout

```text
server/                 the Go server; Tachyon compiles it (package main)
  application.json      Yon settings; every design rule is on
  controllers/          @RequestRoute controllers (one per former route)
  services/             @Service providers
  helpers/              @Helper providers (the former repositories, as XHelper)
  shared/<area>/        plain Go types: route logic, records, validation, protocols
  models/               data-only types used in route signatures
  tests/                Yon test units
cmd/caduceus/           the Go binary (a normal Go module with internal/)
internal/
  runtime/entry/        `caduceus entry`: the container entry point (was scripts/entry.mjs)
  runtime/settings/     HandlerSettings and the .env writer for the handlers
  runtime/supervisor/, runtime/tickworker/, runtime/acmecert/
                        the service supervisor, the queue/snooze/reminder/provider
                        workers and ACME (were background/*.ts, *.js)
  mail/inbound/, mail/submission/, mail/smtpd/, mail/mailauth/
                        the inbound MX, submission and SPF/DKIM/DMARC
  platform/fylo/        the FYLO owner and the one collection list
  platform/fylobridge/, platform/bridgeclient/
                        the signed bridge and its Go client
client/                 the browser client (merged separately)
scripts/                build-go, create-admin and migrate-domain (Bun), e2e-preview
tests/
  routes/, api/, db/    suites that run the Go server over HTTP (tests/support/TestServer.mjs)
  support/legacy-server/  the JavaScript server: reference and seeding container
  e2e/                  Playwright against one `ty serve` (scripts/e2e-preview.mjs)
```

### What is kept in step across server/ and internal/

`server/` is compiled by Tachyon as package main and cannot import
`internal/`, so a few things exist on both sides. Tests keep them in step:

- Bridge frames: `server/shared/boundary/Boundary.go` and
  `internal/platform/bridgeclient` both sign frames for the FYLO owner;
  `internal/platform/fylobridge/frames_test.go` builds the server's Boundary
  and checks the bridge accepts both.
- Settings: `server/shared/runtime/Settings.go` reads any name from the
  environment or `YON_APPLICATION_JSON`; `internal/runtime/settings` owns the
  list the entry point hands over (`HandlerSettings`).
  `server_settings_test.go` fails when the server reads a name not listed, or
  when `tests/support/TestServer.mjs` lists different names.
- Collections: `internal/platform/fylo/collections.go` is the one list the
  entry point creates and the bridge allows. The server names the ones its
  routes use in `server/shared/persistence/FyloCollections.go`;
  `collections_test.go` checks the server's names, the test reset's and the
  legacy container's against it. The seven authentication collections
  (`mfa-devices` ... `auth-audit-events`) are retired (SESAME owns
  authentication) but stay created for existing data roots.
- The inbound webhook: `internal/mail/inbound` posts
  `{recipient, sender, subject, body, spamVerdict?, spamScore?}` with
  `X-Caduceus-Signature`, the hex HMAC-SHA256 of the exact body;
  `SystemSignature` verifies the same bytes. `tests/routes/inbound-smtp.test.js`
  runs it end to end.

Tachyon's `server/packages/` (a normal Go module that controllers and
services may import) is where the frame signer and the settings reader could
later live once, for both sides.

Go rules Tachyon enforces (read the diagnostic help text when one fires):

- No `package` clause; Tachyon compiles everything as `package main`.
  One type per file, the file named after the type. No top-level `const`,
  `var` or `func`: a value is a method that returns it, such as
  `func (FyloAccess) idSpan() int { return 2_000_000_000 }`.
- Standard library only. No third-party modules, no cgo. Code reaches other
  types of the server by name, without imports.
- Annotations are comments on the line before the type or method:
  `// @RequestRoute("/domains")`, `// @SignedIn`, `// @GetRoute("/{id}")`,
  `// @Needs` (on the field before it), `// @Helper`, `// @Service`.
- Route methods take `*http.Request` (and `// @FromPath` parameters before
  it) and return `*http.Response`. Method names start with the verb
  (`GetDomains`, `PostDomain`).
- A composite literal in an `if` condition needs parentheses:
  `if !(Security{}).HasDomainClaim(...)`.
- Design rules: controllers and services hold no control flow in public
  methods (TY1314); repeated literals and numbers in conditions are named
  (TY1330); a member nothing outside uses is lowercase (TY1333); names are
  plain English (TY1328, domain words are listed in `names.words`);
  parameter counts are bounded (TY1329); no duplicated code (TY1315).
  `server/shared/` is plain code and is not held to the cohesion rules, so
  route logic lives there.

## The pattern

A controller delegates to a plain route type in `server/shared/routes/`
that holds the logic with ordinary Go control flow:

```go
// @RequestRoute("/domains")
// @SignedIn
type DomainsController struct {
	// @Needs
	accessHelper *AccessHelper
	// @Needs
	domainsHelper *DomainsHelper
}

// @GetRoute
func (controller *DomainsController) GetDomains(request *http.Request) *http.Response {
	return controller.route().List(request)
}

func (controller *DomainsController) route() DomainsRoute {
	return DomainsRoute{Access: controller.accessHelper, Domains: controller.domainsHelper}
}
```

`server/shared/routes/DomainsRoute.go` and `server/controllers/DomainsController.go`
are the reference slice.

## Foundation (do not change without asking; add new files instead)

- `Settings{}.Value("NAME")` — a deployment setting (yon.* in a handler).
- `Boundary{}.FromSettings()` — the signed frame transport to the Go owner.
- `FyloDatabase` — FYLO operations: `PutData`, `GetDoc`, `FindDocs`,
  `PatchDoc`, `DelDoc`, and `Op(name, fields)` for any other operation. It
  answers documents as `[]FyloDocument{Id, Data}` in FYLO's order: keep that
  order wherever the JavaScript returned `Object.values(docs)`.
  The JSON fields of each operation must match what
  `tests/support/legacy-server/deps/fylo.mjs` sends for the same call (for
  example `patchDoc` sends `{"id": "<docId>", "newDoc": fields}`, one
  operation per document, as the JavaScript `Fylo.patchDoc` wrapper did with a
  `{docId: fields}` object; `FyloCollection.PatchEach` does the same in Go).
- `FyloCollections{}.Named().Emails` — collection and bucket names.
- `FyloAccess` — POSIX access descriptors when `FYLO_POSIX_ACCESS=true`.
- `FyloHelper` (@Helper) — the request's database; `AccessHelper` (@Helper)
  — `Signed(request)` or `Allowed(request, action, domain, mailbox)`, which
  also makes the request's storage act as the caller (the former
  `FyloAccessContext.run`).
- `RouteAccess` — boundary actions (`MailboxRead()`, `DomainAdmin()` ...),
  `Bearer(request)`, `Failure(err)` (the former `AuthGuard.failure`).
- `AuthClient` — `Call(op, params)` for any auth operation, plus
  `Authenticate` and `Authorize`.
- `Answer{}` — `Ok`, `Json(status, v)`, and the error shapes of the former
  `Respond` (`BadRequest`, `Unauthorized`, `Forbidden`, `NotFound`,
  `Conflict`, `Blocked`, `RateLimited`, `Unavailable`).
- `RequestBody{}.Object(request)` / `.Value(request)` — the JSON body, with
  the 400 answer for malformed JSON.
- `Security{}` — address and domain normalisation, webhook URL safety.

## Data compatibility

Documents written by Go must be readable by the JavaScript server and the
other way round: same collection, same field names, same encodings (for
example a domain's `routes` is stored as JSON text). Token formats
(tracking, thread shares, MCP, VAPID) must stay byte-compatible.
Collections are created once by the process that starts the server (the
entry point, or the test container); routes never create them.

## Checking your work

- Build with every rule on:
  `<work>/gocheck.sh <your worktree> <your probe dir>` prints every
  diagnostic (`OK` when there are none). Fix all of them in your files.
- Behaviour: run the route suites that cover your routes against your Go
  server over HTTP:
  `CADUCEUS_SERVE_ROOT=<your probe dir> TACHYON_BIN=<work>/tachyon-target/release/ty bun test tests/routes/<suite>.test.js --timeout 60000`
  (gocheck.sh refreshes the probe's server/ from your worktree first).
  `<work>/baseline-http.txt` lists what each suite scores against the
  JavaScript server over HTTP; match or beat it. Suites that reach into
  in-process fakes (`app.smtp = ...`) or controller statics cannot pass over
  HTTP; list them in your report rather than changing what they assert.
