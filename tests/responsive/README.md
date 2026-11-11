# Mesail responsive and accessibility test

A Go test that drives headless Chromium (through
[chromedp](https://github.com/chromedp/chromedp)) against a built Mesail
app (the web app `apps/web`, or the admin app `apps/web-admin`) and checks the gate of ADR 0004 section 8 ("Responsive and
accessible, as a gate"), SLICES rule 13 and SPEC AC-UI-1.

It is behind the `responsive` build tag, so `go test ./...` and
`go vet ./...` elsewhere skip it.

## Run it

1. Build the app:

       ty bundle apps/web --target web

   For the admin app, build `apps/web-admin` instead and set
   `MESAIL_APP=web-admin` in step 2.

2. Run the test:

       cd tests/responsive
       go test -tags responsive -count=1 ./...

   Add `-v` to also see the warnings (forced colors) and the passing runs.
   Add `-run 'TestEveryWidth/signed-in/375px'` to run one case.

On the shared Mac, run it through the lock and keep the log:

    MAC_HEAVY_COORDINATOR=caduceus-build mac-heavy --owner <name> --light -- \
      go test -tags responsive -count=1 -v ./... 2>&1 | tee ~/Library/Logs/local/CADUCEUS/<run>/responsive.log

Settings (environment variables):

| Variable | Default | Meaning |
| --- | --- | --- |
| `MESAIL_APP` | `web` | The app under test: `web` or `web-admin` (see "Apps" below). |
| `MESAIL_DIST` | `../../apps/web/dist/web` (`../../apps/web-admin/dist/web` with `MESAIL_APP=web-admin`) | The built `dist/web` folder to serve. |
| `MESAIL_CHROME` | newest Playwright Chromium | The Chrome or Chromium binary. |
| `PLAYWRIGHT_BROWSERS_PATH` | `~/Library/Caches/ms-playwright` | Where to look for `chromium-*`, then `chromium_headless_shell-*`. |

The test copies `MESAIL_DIST` to a temporary folder when it starts and
serves the copy, so a rebuild during the run does not change what it tests.

The test skips (with the reason) when no Chromium is found, and fails when
`dist/web/index.html` is missing.

## Apps

`MESAIL_APP` picks one app profile (`AppProfile.go`) when the test starts.
A profile holds everything that differs between the apps: the default
`MESAIL_DIST`, the session storage key and the value stored in it, the
hosted domains and roles of the fake identity API, and the fixed answers for
the app's data calls. To test another app, add one profile to
`AppProfiles()`.

| | `web` (default) | `web-admin` |
| --- | --- | --- |
| Session storage key | `tac:service:session:/:sessionKey` | `tac:service:adminSession:/:sessionKey` |
| Stored value | `"key-example.com"` (one string; Tac stores every field as JSON) | `"key-example.com"` |
| Hosted domains | `example.com`, `example.org` | `example.com`, `example.org` |
| Default domain (the first hosted domain; a login without `@` opens it) | `example.com` | `example.com` |
| Roles | `["mesail-member"]` | `example.com`: `["mesail-admin","mesail-member","mesail-operator"]` (both admin menus show); `example.org`: `["mesail-member"]` (no admin rights) |
| Fixed data answers | none | `GET /api/v1/directory/domains`: `200` with `example.com` (active) and `gone.example.org` (removing); `GET /api/v1/directory/members`: `200 {"items":[]}`; `GET /api/v1/directory/people`: `200` with ana (active), gail (deleted, one domain left: Deleting) and finn (deleted, no domain left); `GET /api/v1/channels`: `200 []` |
| Extra pages (`TestViews`) | none | Domains (waits for the "Removing" tag), People (waits for the "Deleting" tag) |

Both apps use the same shell-ready selector (`nav[aria-label="Domains"]`),
the same sign-in form (the username field `autocomplete="username"`, the
password field, the button "Sign in") and the same form sign-in
(`ana@example.com` / `right`). Since ADR 0005 there is no operator realm:
the admin profile gives the Operator role with the session of a hosted
domain, and a login without `@` (`ana`) opens the first hosted domain.

## What it serves

The test serves `dist/web` itself on a random `127.0.0.1` port, with the
same Content-Security-Policy that TACHYON 26.46.02's server sends (`'self'`,
`'wasm-unsafe-eval'` and the hashes of the page's inline scripts). The same
origin has a fake identity API (`docs/mesail/CONTRACTS.md`, "identity"):

- `POST /api/v1/identity/session`: password `right` gives `200 Session`
  for a hosted domain of the app profile (the part of `login` after `@`,
  or the first hosted domain for a login without `@`), with that profile's
  roles and `domains[]` (every hosted domain, `admin` from the roles,
  `default` on the first). A domain that is not hosted gives `403
  not_a_member`; the login `dan` gives `403 no_domain`, `new` a session
  with `mustChangePassword`, and `op` an Operator session with the domain
  `*`. Password `busy` gives `429`, any other wrong password `401`.
- `POST /api/v1/identity/session/refresh`: `sessionKey` `key-<domain>` gives
  `200 Session` for that domain, anything else `401`.
- `POST /api/v1/identity/session/domain`: `sessionKey` `key-<any>` and a
  hosted `domainId` give `200 Session` for that domain; another key `401`,
  a domain that is not hosted `404`.
- `DELETE /api/v1/identity/session`: `204`.
- The app profile's fixed data answers (admin app only; see "Apps"). Every
  other admin data call (`/api/v1/directory/*`, `/api/v1/channels/*`, any
  `POST`, `PATCH` or `DELETE`) answers `404` for now.
- Any other `/api/` call answers `404`, and the test logs it.

The signed-in shell is reached without typing: the test stores the profile's
value in its session storage key (where Tac keeps the `// @SessionState`
field: `SessionKeys` of the web app's `session` service, `SessionKey` of the
admin app's `adminSession` service) before the app loads, and the app
restores the sessions through `/refresh`. If that restore does not reach
the shell within 20 s, `TestSessionRestore` fails. Its message lists every
console and log entry and every API call. The test then sends one
`visibilitychange` event. If the shell appears after that event, the restored
state was there and only the re-render was missing, and the message says so.
From then on, the other signed-in checks sign in through the form instead
(`ana@example.com` / `right`).

## What it checks

`TestEveryWidth` loads the sign-in screen and the signed-in shell at 320,
360, 375, 414, 768, 1024, 1280, 1440, 1920 and 2560 px wide. At each width:

1. No horizontal scrolling: the document is not wider than the viewport. The
   failure names the elements that stick out past the edge.
2. Touch targets: every visible interactive element (`a[href]`, `button`,
   `input`, `select`, `textarea`, `summary`, interactive ARIA roles, a
   `tabindex` other than `-1`, `contenteditable`) is at least 44 x 44 px
   under 768 px or with a coarse pointer, and at least 24 x 24 px from
   768 px with a fine pointer. A form field's target is its field box: the
   largest ancestor up to its `label` or `w-*` element that holds no text
   other than the field's own label (so a DuVay `w-text-field` is measured
   by its outlined box, not the bare `input`).
3. No clipped text: an element with its own text has a scroll width no
   larger than its client width (+1 px), unless it uses
   `text-overflow: ellipsis`. Visible text boxes of unrelated elements must
   not overlap.
4. AC-UI-1 layout (shell only): under 768 px exactly one of the panes
   (the sidebar `aside`, `main`, the thread `aside`) is visible and the
   `nav` labelled "Domains" is hidden; from 768 px the rail, the sidebar and
   `main` are visible and do not overlap; the thread column is not shown
   (no thread is open in this test).
5. No console errors, uncaught exceptions or Content-Security-Policy
   violations (`/favicon.ico` 404s are ignored).

Extra runs:

- `TestViews`: each extra page of the app profile (`Views`; the admin app
  only). The test signs in, presses the page's menu button and waits for
  its tag. Then it runs the checks of `TestEveryWidth` at every width, in
  light and dark (dark also checks the background), and at 375 and 1024 px
  with a coarse pointer (44 px targets). For the web app it skips.
- `TestCoarsePointer`: 375 and 1024 px with touch and mobile emulation
  (`pointer: coarse`), all checks, 44 px targets.
- `TestReducedMotion`: `prefers-reduced-motion: reduce`; no visible element
  keeps a transition or animation longer than 0.01 s.
- `TestDarkMode`: `prefers-color-scheme: dark`; the `body` (or `html`)
  background has a relative luminance under 0.2.
- `TestRightToLeft`: `dir="rtl"` on `<html>` after load; no horizontal
  scrolling or clipped text at 375 and 1440 px, and the Domains rail is on
  the right half at 1440 px.
- `TestForcedColors`: `forced-colors: active`; each visible focusable element
  is focused in turn and must show a nonzero outline or border (on itself or
  its field box). DuVay has no forced-colors support yet (ADR 0004), so this
  only logs warnings; run with `-v` to see them.
- `TestSafeAreas`: the built CSS uses `env(safe-area-inset-top)` and
  `env(safe-area-inset-bottom)`; and with Chrome's safe-area override (top 47 px, bottom 34 px, at 375 px
  with a coarse pointer) no visible text or control starts in the top band,
  and no pinned (fixed, sticky or non-scrolling) text or control ends in the
  bottom band. A viewport meta without `viewport-fit=cover` is only a logged
  warning: ADR 0004 section 8 ("Safe areas") says Tac pages get no
  `viewport-fit=cover` today, and the shell uses TACHYON's
  `--tac-titlebar-height` with `env(safe-area-inset-*)` as the web fallback.

Every failure names the screen, the width and height, the pointer, and the
element (its ARIA role and accessible name, or its tag and id).

## Limits

- The checks read Chrome's DOM snapshot, accessibility tree and layout
  metrics through the DevTools protocol. The only JavaScript is
  `matchMedia(...)`, to confirm that an emulation took effect, and the one
  `visibilitychange` event in the session-restore diagnosis.
- "Visible" means: laid out, `visibility: visible`, no `opacity: 0` on the
  element or an ancestor, more than 1 px in each direction, inside the
  viewport horizontally and not above it, and not clipped away by an
  overflow ancestor.
- Overlapping text compares inline text boxes and does not know paint order,
  so text covered on purpose by an opaque layer would be reported.
- Contrast and keyboard order (AC-UI-2) are not checked here.
