# ADR 0009: Thin controllers and numbered error codes

- Status: accepted (2026-11-12)
- Deciders: the owner (the rule), architecture (the details)
- Related: ADR 0004 (Go backend), CONTRACTS.md "Common rules", SLICES.md
  rule 18 and Wave 7, `docs/mesail/ERRORS.md`; TACHYON ADR 0051
  (`@ErrorHandler`), ADR 0101 (`@Check` answers 422)

## Context

The owner's rule (2026-11-12), in the owner's words: "the controller
methods in the backend should have at most one line -- returning the result
of the `Service` they're calling. if there is an error, the error handler
... should handle it and return an error message containing a error code
(e.g. [MS1234]: <error message>)."

What is true on `develop` today:

- Every backend app has one `@ErrorHandler` (TACHYON ADR 0051) in
  `server/controllers/<App>ErrorHandler.go`. It turns typed errors into
  `ApiError{code, message, field?}`, with snake-case codes such as
  `invalid_request` that the web apps and the blackbox tests read.
- Controllers do more than one thing. For example `MessagesController.
  PostMessage` finds the caller, calls a helper that branches, and maps
  the result; `IdentityController.GetMe` builds the session code itself.
- A `server/services/` layer exists in only four apps: `directory` (37
  files), `notify` (40), `messaging` (10) and `mail-in` (1). In
  `attachments`, `channels`, `identity`, `mail-out`, `realtime` and
  `search`, controllers call the domain code in `server/packages/<domain>/`
  directly.
- TACHYON 26.46.02 answers some errors itself, before or around the app's
  code, so no `@ErrorHandler` sees them (`docs/YON_ANNOTATIONS.md`, "Who may
  call a route" and "Error handlers"):
  - no identity on a `@SignedIn` or `@Allow` route: `401`, plain text;
  - a missing role: `403`, plain text ("Your roles do not allow this
    route.", found by PK3);
  - the identity provider fails: `500`;
  - a `@Check` rule breaks: `422` problem details (ADR 0101);
  - a malformed JSON body: refused by TACHYON (ADR 0026);
  - an error that no handler method takes, or an error inside a handler:
    the fixed `500`.
- TACHYON has no way to pass the identity that `@IdentityProvider` found
  into a route method. A route reads the caller from the request again.

## Decision

1. **One line per route method.** A controller route method has exactly
   one statement: it returns the result of one method of one Service.
   For Go: `return c.messages.Post(request, body)`. Finding the caller,
   checks, branches, loops, wire mapping and error mapping all move into
   the Service (or the domain code it calls). Socket and stream routes
   follow the same rule. A controller holds only its injected Services.
2. **Every backend app has a Service layer**, in
   `apps/<app>/server/services/<Name>Service.go`, as TACHYON providers
   (the pattern `directory`, `notify` and `messaging` already use). A
   Service is the only thing a controller calls. Services call the domain
   code in `server/packages/<domain>/`, stores and clients, as before
   (dependencies point down, the code rules).
3. **The caller.** Until TACHYON passes the identity to the route (request
   Y2 below), a route passes the native request to the Service, and the
   Service's first step finds the caller with the app's existing caller
   code. When Y2 lands, a mechanical follow-up replaces the request with
   the injected identity.
4. **Numbered error codes.** Every error answer that Mesail code shapes
   carries a Mesail error number in its message:
   `ApiError.message` = `[MS<4 digits>]: <plain English message>`, for
   example `[MS0510]: This passkey name is already used.`
   - `ApiError.code` and `ApiError.field` do not change. Programs (the web
     apps, the blackbox tests, other services) keep reading `code`; people
     and support read the number. This keeps every client working.
   - One number means one failure case in one app, so a support report
     finds the exact place. A number is never reused or renumbered.
   - Each app has its own block of 500 numbers; the first number of each
     block (for example `MS0500`) is "unexpected error" for that app.
     The blocks and every number are listed in `docs/mesail/ERRORS.md`.
   - The text after the number is short, plain English, and never holds a
     token, password, key, file path, address list or message content.
   - The `@ErrorHandler` builds the message; error types carry the number
     (a field or method of the error type). The helper that formats it
     lives in `kit/apierrors` (slice E0) and is copied into each app.
5. **Errors TACHYON answers itself** keep TACHYON's body until TACHYON
   lets the app shape them (request Y1 below). Clients keep the rule of
   CONTRACTS "Common rules": any `401` means "sign in again", whatever its
   body. Mesail's own router (`deploy/router`) and the SMTP edge are
   Mesail code: the router's answers (`429`, `502`, `503`, `504`) get
   numbers from the router's block; SMTP replies keep SMTP's own status
   codes and get no Mesail number.

## Requests to TACHYON

- **Y1 (needed for the rule to cover every answer).** Let an
  `@ErrorHandler` shape the answers TACHYON makes itself: pass each one to
  the handlers as a typed error, and use the handler's answer when the app
  declares a `@HandlesError` method for that type; keep today's answer
  when it does not. At least: no identity (`401`), missing role (`403`),
  identity provider failed (`500`), `@Check` rejection (`422`, with the
  broken rules in the error so the handler can list them), malformed or
  too large body (`400`, `413`), and an error no other handler method
  takes (a catch-all, so the app can answer its "unexpected error"
  number). TACHYON keeps the status code; the handler shapes the body.
- **Y2 (not blocking).** A route method parameter that receives the
  identity the `@IdentityProvider` returned for this request (for example
  `@FromIdentity`), so the controller passes it to the Service and nobody
  reads the token twice.

## Consequences

- Every app's controllers and error handler change, and six apps get a new
  Service layer. Slices E0 to E9 (SLICES Wave 7) do it app by app, after
  the running slices that touch the same files.
- `ApiError.message` gains a fixed `[MSnnnn]: ` start. Nothing reads the
  message text today except tests that compare two whole `ApiError`
  values, which still match when the same failure has the same number.
- Until Y1, the gate answers (`401`, `403`, `422`, malformed body) carry
  no Mesail number. CONTRACTS lists them as exceptions.
