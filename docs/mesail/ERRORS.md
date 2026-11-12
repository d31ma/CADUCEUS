# Mesail error numbers

Every error answer that Mesail code shapes has a Mesail error number in
`ApiError.message`: `[MS<4 digits>]: <plain English message>` (ADR 0009).
`ApiError.code` (for programs) does not change. A number names one failure
case in one app. Numbers are never reused or renumbered; a retired case
keeps its row, marked "retired".

## Blocks

| Numbers | Owner | First number ("unexpected error") |
| --- | --- | --- |
| MS0000-MS0499 | shared `kit` code (for example `kit/oidc`, `kit/outbox`) | MS0000 |
| MS0500-MS0999 | `identity` | MS0500 |
| MS1000-MS1499 | `directory` | MS1000 |
| MS1500-MS1999 | `messaging` | MS1500 |
| MS2000-MS2499 | `channels` | MS2000 |
| MS2500-MS2999 | `mail-in` | MS2500 |
| MS3000-MS3499 | `mail-out` | MS3000 |
| MS3500-MS3999 | `attachments` | MS3500 |
| MS4000-MS4499 | `search` | MS4000 |
| MS4500-MS4999 | `notify` | MS4500 |
| MS5000-MS5499 | `realtime` | MS5000 |
| MS5500-MS5999 | the router (`deploy/router`) | MS5500 |
| MS6000-MS9999 | reserved; architecture assigns a new block | - |

## Rules

- Each slice of Wave 7 (SLICES.md) adds its app's rows here in the same
  commit as the code, sorted by number.
- Numbers inside a block go in order of first use; leave no gaps on
  purpose.
- **One number per case, not per error type** (decided 2026-11-12, from
  E1). When one error type has several cases (for example an upload with
  a bad file name, media type, size or checksum), each case has its own
  number. The error type's file offers one constructor per case (for
  example `InvalidUploadFileName()`), and the code that raises the error
  calls the constructor of its case, so no Service keeps a table from
  fields to numbers. `field` still names the field.
- The catch-all is a `@HandlesError` method that takes plain `error`
  (TACHYON: a concrete error type wins over it): status 500, `code`
  `unavailable`, the block's first number.
- The message text is short, plain English, and never holds a token,
  password, key, file path, address list or message content.
- Each app's tests (built on `kit/apierrors`, slice E0) check that every
  error answer has its exact number and that the number is inside the
  app's block. `ty test` runs app tests in a temporary folder without the
  repository, so it cannot read this file; the gate check of slice E11
  checks this file against the code instead: every number listed once,
  inside its app's block, and every number the code uses listed here.
- Shared kit packages that are copied into one app only (for example
  `kit/channels`) use that app's block, and their error types carry the
  numbers, one constructor per case.
- Answers that TACHYON makes itself (`401` without identity, `403` without
  a role, `422` `@Check` rejections, a malformed body, an error no handler
  takes) have no number until TACHYON request Y1 (ADR 0009).

## Numbers

| Number | App | Status | `code` | Message | Case |
| --- | --- | --- | --- | --- | --- |
| MS0000 | kit | 500 | `unavailable` | Something went wrong. | an unexpected error in shared code |
| MS0500 | identity | 500 | `unavailable` | Something went wrong. | an unexpected error in `identity` |
| MS1000 | directory | 500 | `unavailable` | Something went wrong. | an unexpected error in `directory` |
| MS1500 | messaging | 500 | `unavailable` | Something went wrong. | an unexpected error in `messaging` |
| MS2000 | channels | 500 | `unavailable` | Something went wrong. | an unexpected error in `channels` |
| MS2500 | mail-in | 500 | `unavailable` | Something went wrong. | an unexpected error in `mail-in` |
| MS3000 | mail-out | 500 | `unavailable` | Something went wrong. | an unexpected error in `mail-out` |
| MS3500 | attachments | 500 | `unavailable` | Something went wrong. | an unexpected error in `attachments` |
| MS4000 | search | 500 | `unavailable` | Something went wrong. | an unexpected error in `search` |
| MS4500 | notify | 500 | `unavailable` | Something went wrong. | an unexpected error in `notify` |
| MS5000 | realtime | 500 | `unavailable` | Something went wrong. | an unexpected error in `realtime` |
| MS5500 | router | 502 | `unavailable` | Something went wrong. | an unexpected error in the router |
