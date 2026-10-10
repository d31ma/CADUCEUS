# Porting the CADUCEUS server to Go

The server is being rewritten in Go on Tachyon Yon. The JavaScript server it
replaces is kept at `tests/support/legacy-server/` as the reference for
behaviour and as the test suites' way to seed records. Port it faithfully:
same routes, same request and response JSON, same FYLO documents, same error
messages and statuses. This is a rewrite of the language, not of the product.

## Layout

```text
server/
  application.json     Yon settings; every design rule is on
  controllers/         @RequestRoute controllers (one per former route)
  services/            @Service providers
  helpers/             @Helper providers (the former repositories, as XHelper)
  shared/<area>/       plain Go types: route logic, records, validation, protocols
  models/              data-only types used in route signatures
```

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
  example `patchDoc` sends `{"id": {"<docId>": fields}}`).
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
  `<scratch>/gocheck.sh <your worktree> <your probe dir>` prints every
  diagnostic (`OK` when there are none). Fix all of them in your files.
- Behaviour: run the route suites that cover your routes against your Go
  server over HTTP:
  `CADUCEUS_SERVE_ROOT=<your probe dir> TACHYON_BIN=<scratch>/tachyon-target/release/ty bun test tests/routes/<suite>.test.js --timeout 60000`
  (gocheck.sh refreshes the probe's server/ from your worktree first).
  `<scratch>/baseline-http.txt` lists what each suite scores against the
  JavaScript server over HTTP; match or beat it. Suites that reach into
  in-process fakes (`app.smtp = ...`) or controller statics cannot pass over
  HTTP; list them in your report rather than changing what they assert.
