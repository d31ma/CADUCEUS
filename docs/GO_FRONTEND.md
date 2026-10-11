# The Go browser client

The CADUCEUS browser client runs on Tachyon Tac with its behaviour in Go
services. This page records the conventions the client follows, so every
area of the client is built the same way. Each rule says what to do and why
Tachyon requires it. Each one was checked against `ty bundle` with every
design rule on.

## Layout

| Folder | What lives there | Rule |
| --- | --- | --- |
| `client/pages/**/tac.html` | Component tags only | A page holds no markup, no `tac.css` and no `tac.js` (TY1309). |
| `client/components/<area>/<name>/` | `tac.html` and `tac.css` | A component holds no script (TY1310). `tac.css` is wrapped in `@scope (tachyon-component[data-tac-scope="<area>-<name>"]) to (tachyon-component[data-tac-scope])`, so it styles this component only. It does not style nested components. |
| `client/services/<Name>Service.go` | One `// @Service` struct and its methods | The view names it `<name>`. For example, `MailboxService` is `mailbox`. |
| `client/services/<Name>Service.js` | Escape hatches only | Use this only for a browser capability that Go cannot reach. See below. |
| `client/remotes/<Area>Client.go` | One `// @HTTPClient("/<route group>")` interface for each server route group | Write only the methods your area calls. |
| `client/setups/<Area>Setup.go` | `Configure` returns `NewBehaviorConfiguration(), nil` | With no adapter, the client uses the browser transport to the page's own origin. |
| `client/models/<Model>.go` | One struct for each file, with `json:"camelCase"` tags | An event that a JavaScript bridge publishes or hears also needs a JSDoc twin, `client/models/<Model>.js`. |
| `client/shared/utils/<Name>Utils.go` | Pure logic that several services share | Use `type XUtils struct{}` with value-receiver methods. A utility holds no state. |
| `client/shared/scripts/page-setup.js` | Page-wide browser setup | It is linked into every page by `tac.scripts`. |

## Go services cannot inject each other

Each Go service is compiled into its own Go package. Its program includes its
remotes, its setups, the models and the utilities, but no other service. A
field such as `// @Needs\nsessionService *SessionService` therefore fails
with `undefined: SessionService`. Services share state through application
events instead:

- The owner publishes a **retained** event:
  `applicationEventPublisher.PublishEvent(SessionChanged{...}, true)`.
- Every listener uses `// @OnEvent({"replay":true})`, so it also hears the
  value that was published before it started.
- Two components that name the same service share one page instance.

A view can name any service directly, for example `{session.displayName}`.
Tachyon creates a service only when some view names it. A service that no
view names never runs. Add a hidden binding when needed, for example
`<span hidden :data-browser="browserPage.ready"></span>`.

## Calling the server

```go
// @HTTPClient("/users")
type UsersClient interface {
	// @GetRequest("/me/preferences")
	Preferences(
		// @RequestHeader("Authorization")
		authorization string,
	) (UserPreferences, error)

	// @PutRequest("/me/preferences")
	SaveTheme(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @RequestBody
		theme BehaviorValue,
	) (BehaviorValue, error)
}
```

- **Request bodies.** A `@RequestBody` parameter is `BehaviorValue`. Pass a
  model value, and it is sent as JSON.
- **Typed and untyped methods.** A method that returns `(Model, error)` may
  take `string`, `int`, `float64` or `bool` parameters. A method that returns
  `(BehaviorValue, error)` takes only `BehaviorValue` parameters (TY1308).
- **Authorization.** Send `"Bearer " + token` in a
  `@RequestHeader("Authorization")` parameter. Read the token from the
  retained `SessionChanged` event.
- **Where HTTP may run.** HTTP runs only inside a call that the view started
  or inside an `@OnEvent` handler. It cannot run in `@OnStart` or in a
  goroutine. To load data at startup, listen for `SessionChanged` (with
  `replay`) and load in that handler.
- **Rendering after event work.** The page renders again after a view call
  finishes. After an `@OnEvent` handler that waited on HTTP, the page renders
  again only when the handler publishes an event. End such a handler with a
  publication, for example `FolderCountsChanged`, `TeamRefreshed` or
  `LoginProgressed`.
- **Failures.** The browser transport reports every error status as
  `HTTP_STATUS: The browser HTTP request failed.`. It hands over no status
  code and no body. Show a fixed message:
  `HTTPUtils{}.Message(failure, fallback, network)`. For a refused
  authenticated read, publish `SessionCheckRequested`. SessionService then
  asks `GET /auth/session` and signs out only when the token is dead. This
  replaces "401 means sign out" without signing out on a 404 or a 500.
- **Origin.** The transport is same-origin only. `CADUCEUS_CONFIG.apiUrl`
  (a separate API origin) cannot be used.
- **Limits.** A request may hold 64 KiB and an answer 1 MiB. A larger answer
  fails with `HTTP_LIMIT`. A JavaScript service may not call `fetch` either
  (TY1361). Uploads larger than 64 KiB therefore have no route today. These
  include EML and mbox import, the PWA file handler, and composer
  attachments. Neither does an authenticated server-sent event stream, such
  as `/inbox/stream`.

**Blocker for the merge.** On one origin, the server's API routes `/inbox`,
`/activity`, `/contacts` and `/reminders` answer the document navigation to
the page of the same path. The server answers `401 Sign in to call this
route.`. Until the API moves under a prefix, or Tachyon serves the page for
an HTML navigation, those pages cannot load on one origin.

## Saved fields are kept per route

`@SessionState` and `@LocalState` fields are stored under
`tac:service:<instance>:<location.pathname>:<field>`. A value saved on
`/inbox` is not read on `/settings`. Use them only for state that belongs to
one route, such as the login form's step.

State that must survive a navigation goes through the storage bridge
(`BrowserStorageService.js`):

- At start, every `caduceus*` key of sessionStorage and localStorage arrives
  in the retained `StoredValues{Session, Local}` event.
- To write, publish `StorageWritten{Area: "session"|"local", Values, Removed}`.
- The legacy key names stay. They include `caduceus_token`,
  `caduceus_email`, `caduceus_domains`, `caduceus_mail_preferences` and
  `caduceus-theme-preferences`. The e2e helpers seed them, and the unported
  pages read them.

## The session contract (SessionService)

| Need | How |
| --- | --- |
| Who is signed in | Hear `SessionChanged{SignedIn, Token, Email, Name, DisplayName, Role, Domains, RefreshToken}` with `@OnEvent({"replay":true})`. A view reads `session.signedIn`, `session.email`, `session.displayName`, `session.role`, `session.domains`, `session.restored`. |
| Call the API | `"Bearer " + changed.Token` in the `Authorization` header. |
| Sign out | Publish `SignOutRequested{Reason}`, or use a button with `data-sign-out`, or `shell.signOut()` in the shell. SessionService posts `/auth/logout` with the ID token, clears the keys, publishes `SessionChanged{SignedIn:false}` and opens `/`. |
| A token may be dead | Publish `SessionCheckRequested{Reason}`. |
| Rotate the token | Publish `SessionRefreshRequested{ClientID}`. The new token arrives in `SessionChanged`. |
| Complete a login | The login form publishes the `TokenSet` it received. SessionService stores it and opens `/inbox`. |

## Events that replace the old `window` CustomEvents and `window._caduceus`

| Old | New event model | Owner or listener |
| --- | --- | --- |
| `caduceus:refresh-inbox` | `InboxRefreshRequested{Reason}` | MailboxService recounts. The inbox list should refresh. |
| `caduceus:folder-counts` | `FolderCountsChanged{Counts, ByDomain, ByAccount, UnreadInbox, UnreadInboxByDomain}` | Publish it after counting. MailboxService shows the badges. |
| `caduceus:mail-action` | `MailActionApplied{ActionID, ExpiresAt}` | MailboxService publishes it. |
| `_caduceus.performMailAction(ids, patch, message)` | `MailActionRequested{Ids, Patch, Message}` | MailboxService applies it and offers Undo and Redo. |
| `_caduceus.reportSpam` / `markNotSpam` | `SpamReportRequested{ID, Spam}` | MailboxService |
| `_caduceus.toast` / `toastAction` | `ToastRequested{Message, ActionLabel, Action, Subject, DurationMs}` | ShellService shows it. A chosen action comes back as `ToastActionChosen{Action, Subject}` to whoever owns `Action`. |
| `_caduceus.navigate(path)` | `NavigationRequested{Target, Replace, Reload}` | BrowserPageService. Tachyon serves each route as its own document, so a navigation is a document load. |
| theme, font, locale, mail-preference setters | `PreferencesChosen{InterfaceFont, ComposerFont, ThemePreferences, InterfaceLocale, MailPreferences, AccentColor}` | PreferenceService applies and stores it. The loaded values are in the retained `PreferencesLoaded`. |
| `_caduceusKeyboard.applyOverrides` | `KeyboardOverridesChanged{Overrides}` | KeyboardService.js hands it to `keyboard.js`. |
| `caduceus:shortcut:<command>` | `ShortcutPressed{Command}` | KeyboardService.js publishes it for the shell's commands. Other areas add theirs to `KeyboardService._COMMANDS`. |
| `caduceus:external-mailboxes` | `ExternalMailboxesChanged{Accounts, Folders}` (retained) | ExternalMailboxService |
| `caduceus:activity-viewed` | `ActivityViewed{At}` | MailboxService clears the Activity badge. |
| `_caduceus.compose(prefill)` / `consumeComposePrefill()` | Store the prefill JSON under `caduceus_compose_prefill` (`StorageWritten`) and navigate to `/compose`. On `/compose`, ShellService publishes the retained `ComposeLaunched{Fields}`: the stored prefill merged with a mailto or share-target launch (`LaunchUtils`). | The composer hears `ComposeLaunched` with `replay`. |
| `_caduceusUnreadBadge` | `AppBadgeChanged{Count}` (retained) | MailboxService publishes it from the counts and the badge preference. DeviceService.js applies it. |
| service-worker notification buttons | `NotificationActionReceived{Action, EmailID}` | DeviceService.js publishes it. MailboxService applies `mark-read` and `archive`. |

`window._caduceus` and the old globals no longer exist. The e2e suites that
call `window._caduceus.apiFetch`, `performMailAction`, `setMailPreferences`
or `mailPreferences` must change.

## Browser-only pieces

Go services may use only Tachyon's browser APIs: `Clock`, `SecureRandom`,
`Connectivity`, `PageVisibility`, `Preferences`, `Clipboard`, `Share`,
`Notifications` and `Geolocation`. Declare each with `// @Needs`, as in
`notifications Notifications`. Do not call `SecureRandom` from an event
listener: in the TinyGo module it crashed the page ("null function", "memory
access out of bounds"), and so did importing `crypto/rand`. Use
`IdentityUtils{}.UUID(seed)` (a digest of the change and the time) for
request identifiers. Do not write a regular expression with a large
repetition count such as `{1,200}`: TinyGo compiles it into a program deep
enough to overflow a page call's 64 KB stack, with the same crash. Check
characters and length with `TokenUtils{}.Fits` instead. The built-in events `ConnectivityChanged`,
`VisibilityChanged`, `PreferencesChanged` and `NotificationPermissionChanged`
need no model.

Everything else goes through a small JavaScript service. The service only
reads the DOM, makes no decision and talks to Go in events:

| Bridge | What it does |
| --- | --- |
| `BrowserStorageService.js` | `StoredValues` / `StorageWritten`, as described above. |
| `BrowserPageService.js` | `PageLocation` and `BrowserFeatures` at start. `NavigationRequested`, `DocumentAttributesChanged` (attributes on `<html>`, replayed from a cache before Go loads), `TimerRequested`/`TimerElapsed`, `FocusRequested{Selector, Remember, Restore}` and `ClickRequested`. A global Escape becomes `EscapePressed` and a click becomes `PageClicked`. View helpers: `browserPage.dragOver/dragLeave/drop(folder, $event, domain)` → `FolderDropped`, `browserPage.backdrop($event, dialog)` → `BackdropClicked`, `browserPage.submit($event, form)` → `FormSubmitted`. |
| `PasskeyService.js` | WebAuthn: `PasskeyRequested` → `PasskeyAnswered`. |
| `KeyboardService.js` | The bridge to the keyboard runtime in `shared/scripts/keyboard.js`. |
| `DeviceService.js` | The Badging API (with the "(n)" tab-title fallback), the push worker `shared/service-worker/unread.js` registered on its own scope `/shared/service-worker/`, notification buttons, and update detection (`UpdateReady`). Tachyon generates the root worker and no longer lets an application append to it. A push subscription must therefore come from `navigator.serviceWorker.getRegistration('/shared/service-worker/')`, not from `ready`. |

Tac scripts may not build markup or write styles (TY1364). Express a style
choice as an attribute on `<html>` with a CSS rule keyed on it. For example,
`data-accent`, `data-interface-font` and `data-interface-scale` are keyed in
`shared/assets/themes.css`. Bind a style in the template with `:style`, as in
`:style="'--external-account-color:' + account.color"`. To download, set
`:href`/`:download` on a hidden link and publish `ClickRequested`.

## The view

- **Service fields.** A view reads a service by its instance name:
  `{shell.toastMessage}`, `:class="shell.sectionClass('favorites')"`,
  `on:click="shell.toggleUserMenu()"`. Go field `ToastMessage` is
  `shell.toastMessage` in the view.
- **Assignments.** `on:input="login.email = $event.target.value"` sets a Go
  field directly.
- **Arguments.** Methods called from the view take strings, numbers and
  booleans. A Go method does not receive `$event`. Pass
  `$event.target.value`, or let a bridge read the event.
- **Values for loops.** Compute them in Go. Prefer fields on the loop items,
  such as `item.classes` or `providerFolder.href`, over methods that take a
  loop item.
- **The signed-out gate.** The shell gates the signed-out page. A page is
  `<app-shell><your-component /></app-shell>`. When nobody is signed in, the
  shell shows the login screen in place of the slot. The old
  `canUseApp`/`canShowLogin` getters are gone.

## Styles

The global sheets (`tac.styles`) accept only a limited set of selectors and
declarations:

- **Selectors:** `:root`, `html`, `body`, a root attribute such as
  `:root[data-theme="dark"]`, a universal reset or a plain HTML element.
- **Declarations:** colours, fonts, `--*` variables and zero resets.

Everything else moved out of `shared/assets/styles.css`: DuVay host tweaks
(`w-btn`, `w-text-field`), focus rings, `.empty-state`, `.spinner`,
`.text-muted`, the scrollbar and the screenshot masking.

Because component CSS is donut-scoped, **each component copies the rules it
uses into its own `tac.css`**. The shell, the login form and the team roster
already carry theirs. Inside a component, a rule may still key on the root
element: `:root[data-screenshot-mode="true"] img { ... }` works within the
scope.

## Rules that shape every Go file

These run with every check on:

- **Cohesion (TY1314).** A public method only coordinates. It holds no
  `if`, `for`, `switch`, `&&`, `||` or `?:`. A private method holds at most
  one decision or loop. A deciding private method may not call another
  deciding private method of the same class. Patterns that keep code
  readable within these rules:
  - Return the next step: `FlowUtils{}.When(cond, next)()` or
    `FlowUtils{}.Either(cond, a, b)()`.
  - Check a list of conditions: `!slices.Contains([]bool{a, b, c}, false)`.
  - Use a lookup table: `map[string]func(){...}[command]`.
  - Move decisions into a `*Utils` class, because calls across classes are
    not counted.
- **Named values (TY1330).** A literal used twice needs a name. Use one
  method that returns a names model, as in `func (ShellService) names() ShellNames`,
  rather than many one-line constant methods: those count as duplicated code
  (TY1315).
- **Composite literals in conditions.** In an `if`, `for` or `switch` head,
  a literal must be in parentheses: `if x == (LocaleUtils{}).english() {`.
  Otherwise Go reads it as the block.
- **Names (TY1328).** Plain English names. Domain words are listed in
  `tac.names.words`. A parameter named `open` or `protected` is reported as
  `bool`, so rename it.
- **Parameters (TY1329).** At most four. Group the rest into a model.
- **Public members (TY1333).** A public member that no view and no other
  class uses must be private.

## Tests

Go test units live in `client/tests/<Name>Test.go` with a `// @TestSetup` in
`client/tests/<Name>TestSetup.go`. Run them with `ty test`. A case holds no
branch or loop (TY1339). Use `// @ValueSource([...])` for several inputs.
Give browser APIs their answers with `BrowserFakes{}.Answer(api, call, value)`.
To test a listener, call it with a model value.
