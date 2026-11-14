# Mesail UX specification

- Status: Draft for implementation
- Date: 2026-11-10 (one login, ADR 0005, offline first, ADR 0006, and browser push, AC-PN-3: 2026-11-11;
  passkeys and the app lock, ADR 0008: 2026-11-12; audio and video calls, ADR 0012: 2026-11-14)
- Owner: Senior UX/UI Designer (Mesail rewrite)
- Product spec: [../SPEC.md](../SPEC.md) (scenarios S1-S18, acceptance criteria AC-*; one login: AC-ID-1 to AC-ID-14; offline first: AC-OF-1 to AC-OF-20, section 15; passkeys and the app lock: AC-ID-15 to AC-ID-19, AC-OF-20, section 17; calls: AC-CL-1 to AC-CL-9, section 18)
- Design tokens: [TOKENS.md](TOKENS.md)
- Component inventory and Tac mapping: [COMPONENTS.md](COMPONENTS.md)
- Mockups: [mockups/index.html](mockups/index.html) (open the file in a browser; add
  `?theme=dark|high-contrast`, `?os=macos|windows|linux|ios|android`, `?density=compact` or, on a
  window under 600 px, `?pane=list|main|thread`). Built with DuVay v26.35.01-1 web components and
  Tailwind CSS 4.3.5; rebuild the vendored files with `mockups/build/build.sh`.

This document says how Mesail looks and behaves. The Go Tac frontend slices build against it.
Where this document and SPEC.md disagree, SPEC.md decides behaviour and this document decides
presentation; open questions are listed in section 14.

## 1. Principles

1. **One place for every conversation.** Chat and email live in the same sidebar, timeline and
   composer. Email is never a separate app or tab.
2. **Always say how a message travels.** Before sending, every recipient shows *Chat* or *Email*.
   After sending, every message shows its delivery state. The user is never surprised that
   something went out as email.
3. **Chat stays inside a domain. Email crosses domains.** The interface explains this rule in plain
   words wherever a user meets it.
4. **Plain words.** Short sentences, common words. Many users read English as a second language.
   Errors say what happened and what to do next.
5. **Accessible and responsive by default.** WCAG 2.2 AA, full keyboard use, 360 px to desktop.

## 2. Domains (the messaging space)

Each hosted mail domain is its own messaging space (the user's rule). `example.com` and
`example.org` are two separate spaces, even inside one organization. Since ADR 0005 a person has
**one login** for all of them: the username is the email prefix (`ana`), the same in every domain.

- **One sign-in, many domains.** After sign-in, `Session.domains` lists every domain of the person.
  The tab opens the default domain (or the domain of the full address typed at sign-in). A switch
  needs no password and changes only the current tab; a second tab may show another domain.
- **Rail (left edge, 960 px and wider):** one tile per domain in `Session.domains`. The tile shows two
  letters, a mention count or an unread dot. The current domain has a bar on its left and an inverted
  tile. There is no "+" tile: an admin adds a person to a domain, and the new domain appears by itself
  within a minute (at the next refresh).
- **Domain menu** (the domain name in the sidebar header; a bottom sheet under 600 px): "Your domains",
  the **default domain first**, then the others in `Session.domains` order. Each row: tile, domain,
  the address `ana@<domain>`, a **Default** tag, an **Admin** tag when `DomainChoice.admin` is true, and
  the count. Then "Choose your default domain" (settings) and one **Sign out** ("Ends your session in
  every domain"). No per-domain sign-in or sign-out any more.
- **Switching** (`POST /identity/session/domain`): the chosen row shows a spinner and the other rows
  are disabled; after the answer the header, rail, sidebar and address change, the tab opens a new
  realtime connection, and a polite status says "Now in example.org." `404` (removed meanwhile): the
  domain leaves the list with "You are no longer a member of example.org." A refresh that answers `401`
  because the open domain was removed shows "You no longer have access to example.org" with buttons
  for the other domains; when no switch succeeds, the sign-in form.
- **Identity per domain:** Ana is `ana@example.com` in one space and `ana@example.org` in the other.
  Her name, password and 2-step code are the same everywhere; presence and notifications are per domain.
- **Who can be chatted with:** only members of the current domain. Anyone else (another hosted
  domain, or outside) gets email. The recipient chip, the profile card and the composer summary
  all say why: "Chat stays inside example.com. finn@example.org gets an email."
- Mockups: `messenger.html`, `domain-switcher.html`, `domain-switcher-mobile.html`,
  `domain-switch-states.html`, `cross-domain.html`.

## 3. Layout

Breakpoints are DuVay's (`--w-bp-*`), set as the Tailwind breakpoints: `sm` 600 px, `md` 960 px,
`lg` 1280 px, `xl` 1920 px, `2xl` 2560 px. Panes respond to their own width with container queries, so
the same component works in the conversation, the thread pane and a phone screen.

**The apps use one step at 768 px** (decided on 2026-11-11, "Keep 768 px"). Under 768 px `apps/web` and
`apps/web-admin` show one pane at a time; from 768 px they show the rail, the sidebar and the conversation.
The 1280, 1920 and 2560 px rows below apply as written. The 600 and 960 px rows describe the mockups only.
TOKENS.md, section Breakpoints, has the table for the apps.

| Width | Layout |
| --- | --- |
| under 600 px (phones) | One pane at a time: list, conversation or thread (`data-pane`). A back button in each header. Bottom navigation on the list and on tab screens: Home, Activity, Search, You. Dialogs become bottom sheets. |
| 600-959 px (large phones in landscape, small tablets) | Sidebar (272 px) and conversation side by side. No rail: the domain menu in the sidebar header switches domains. The thread or details pane covers the conversation from the right (400 px, shadow) and closes with Esc. |
| 960-1279 px (tablets in landscape, small laptops) | Domain rail (68 px), sidebar, conversation. The right pane still covers the conversation. |
| 1280-1919 px (laptops, desktops) | Rail, sidebar, conversation, and the thread or details pane docked (400 px). |
| 1920-2559 px (large desktops) | Same, with wider panes: rail 72, sidebar 304, right pane 448 px. Message bodies stay at most 80 characters wide. |
| 2560 px and wider (very wide and 4K at 1x) | Sidebar 336, right pane 544 px, and the whole rem-based UI scales to 112.5 % so text stays readable at desk distance. |

- **Short windows** (height under 520 px, phones in landscape): headers shrink to 44 px and hint lines
  hide (Tailwind variant `short:`).
- The sidebar sections, in order: Activity, Threads, **Channels**, **Direct messages**, **Email**.
  The Email section holds direct conversations that include anyone outside the domain. Channels
  with an email address show a small mail icon.
- Sidebar row states: default, hover, focus, current (`aria-current="page"`), unread (bold),
  mention count (red badge), neutral count (grey badge), muted (dimmed, bell-off icon).

## 4. Conversations and messages

- **Timeline:** day dividers, a red "New" divider at the read position, consecutive messages by
  one author grouped (time shows on hover), system lines (joins, channel changes).
- **Message actions** (hover, focus, or long-press on touch): react, reply in thread, edit,
  more (copy link, mark unread, delete). On touch they open as a bottom action sheet.
- **Threads:** one level deep. The root shows "N replies · Last reply hh:mm" with the last
  replier's avatar. The thread composer offers "Also send to #channel".
- **Mentions:** `@name` is tinted; a mention of the current user uses the warning tint and the
  whole message gets a light highlight.
- **Delivery line** under the author's own messages (AC-CP-7): chat *Delivered* / *Seen by …*;
  email *Queued* (clock), *Sent* (check), *Retrying* (amber, with next try time), *Bounced* or
  *Failed* (red, with reason and actions *Edit address* / *Send again*). Each state has an icon and
  a word, never colour alone. "Delivery details" lists every recipient. Mockup: `delivery-status.html`.
- **Edit and delete:** an edited message shows "(edited)". Deleting a message that went out as
  email warns: "Carol Diaz already got this as an email. Email cannot be recalled." (AC-MS-3).

## 5. The one composer

The composer is the same component everywhere. What it shows depends on its recipients.

### 5.1 Recipients and chips (AC-CP-1, AC-CP-2)
- In a channel or an existing conversation, recipients are fixed and the To row is hidden.
- In **New message**, the To row takes names or addresses. Suggestions are grouped:
  "In example.com · Chat" and "Email". A typed address that matches nobody offers
  "Send email to x@y.z".
- Each recipient becomes a **chip**: avatar or initials, name, and a *how* label:
  - **Chat** (teal, chat icon): a member of the current domain.
  - **Email** (violet, mail icon): another domain or an outside address.
  - **Chat + email**: the user chose both for a member.
  - **Checking…** (spinner) while the server resolves the address.
  - **Not valid** (red) with the reason; **Bounced before** (amber) as a warning only.
- Activating a chip (click, Enter or Space) opens its menu: *Chat*, *Email*, *Chat and email*
  (radio items), and *Remove*. For anyone outside the domain, *Chat* is shown but disabled, with the
  reason. Keyboard: Enter adds the typed recipient, Backspace in the empty input selects the last
  chip, arrow keys move between chips and menu items, Delete removes a selected chip.
- Mockups: `compose-new.html`, `compose-chat.html`, `compose-chip-menu.html`.

### 5.2 Email fields (AC-CP-3, AC-CP-6)
When at least one delivery is email, the composer turns into email mode:
- a violet mode bar: "This is an email. Carol Diaz is outside example.com.";
- **From** (primary address or an alias; the channel address in a channel thread);
- **Subject** (required for a new conversation; "Re: …" prefilled and collapsed for a reply);
- **Cc** and **Bcc** buttons that open their rows (resolved like To; Bcc is never shown to others);
- attachments (chips with progress), the signature block, and a violet **Send email** button.
When the last email recipient is removed, the email fields close again; typed subject text is kept
until the composer is cleared.

### 5.3 Summary before send (AC-CP-4, S4)
A one-line summary sits above the toolbar whenever recipients are set:
"Bo Lee gets a chat message. Carol Diaz and Finn Moss get an email. Bo Lee is also on Cc so
replies reach him." The send button names the outcome for mixed sends: "Send · 1 chat, 2 emails".
Mockup: `compose-mixed.html`.

### 5.4 Replies in email conversations
The composer opens in reply-as-email mode with "Reply all" or "Reply to Carol only", subject
collapsed, quoted text hidden behind a toggle. In a channel thread that started from an email,
a reply goes to the outside sender only when "Send to customer" is ticked (AC-CP-8); otherwise it
is marked *Internal*. Mockups: `compose-reply-email.html`, `channel-email.html`.

### 5.5 Composer errors (AC-CP-5, AC-UI-4)
Invalid address, missing subject (focus moves to the field), attachment over the limit
(25 MB per file, 50 MB per message), offline (kept as a draft, sends on reconnect), and the
server's "recipients changed" answer (a dialog lists what changed; nothing was sent; the user
reviews and sends again). Mockup: `compose-errors.html`.

## 6. Email as a conversation (S3, S6-S8)

- An email conversation looks like a direct conversation with an **Email** marker in the sidebar,
  the header and the participant list. Outside participants have violet avatars.
- Each email message is a card inside the normal message row: a violet left rule, subject,
  From/To/Cc, a sender check (verified, or a warning when SPF/DKIM/DMARC fail, AC-IN-4),
  the sanitized body, a "Show quoted text" toggle and attachments.
- Remote images are blocked until the user allows them for the sender (AC-IN-5).
- Internal chat messages between members inside an email conversation are marked
  "Only people in example.com see this".
- Mockups: `email-conversation.html`, `email-inbound-new.html`, `channel-email.html`,
  `email-original.html`.

## 7. Search, activity, presence

- **Search** opens as a palette (Ctrl or Cmd + K) and as a full results page with filters
  (In, From, Date, Has file, Is email). Matches are highlighted with `<mark>`. Results are newest
  first with "Load more" (cursor paging). Only conversations the user can read now are searched.
- **Activity** lists mentions, DMs, thread replies and new email, with tabs and "Mark all as read".
- **Presence** has four shapes as well as colours: active (filled dot), idle (crescent),
  do not disturb (bar), offline (ring). The status menu sets do not disturb with an end time.
- Mockups: `search-palette.html`, `search.html`, `search-states.html`, `activity.html`,
  `presence-menu.html`, `profile-card.html`.

## 8. Sign-in (Mesail's own form)

The browser never talks to Keycloak. Mesail shows its own form and posts it to the identity service,
which checks it with Keycloak (one realm, `mesail`). Mesail stores no password, TOTP secret or
recovery code (AC-ID-5). Mockups: `signin.html`, `password-change-required.html`, `signed-out.html`,
`session-expired.html`; the admin app: `admin-signin.html`.

- **One card** (`signin.html`): **Username or email address** (one field; `type="text"`,
  `inputmode="email"`, `autocomplete="username"`, no auto-capitalization, placeholder "ana or
  ana@example.com"), Password (with a Show button), optional 2-step code ("Only if your account uses
  one."), "Keep me signed in on this device" (offline-first, section 15: off by default in a browser,
  on in the Mesail app), "Remember my username on this device" (off by default; ticked and disabled
  while "Keep me signed in" is ticked), **Sign in**, and "Forgot your password? Ask your admin to reset it."
- **The hint under the field** is computed in the browser from the text only, so it never reveals
  whether an account exists: empty "Your username, like ana, or your full address."; a username
  "Opens your default domain."; a full address "Opens example.org.". The browser trims the text; the
  server lowercases it (`ANA` works).
- **Check before sending:** an empty field, or text that ends in `@`, shows "Type your username, like
  ana, or your full address, like ana@example.com." Nothing else is checked in the browser.
- **401, one answer for every failure** (unknown username, wrong password, wrong code): "The username,
  password or 2-step code is not right. Check them and try again." The password field is cleared and
  focused.
- **403 `not_a_member`** (right password, an address in a domain the person is not in): a warning box
  "You are not a member of partner.test" with **Sign in as ana** (sends the same form again with the
  username only) and **Change the address**. The code field is cleared, because Keycloak does not take
  the same code twice.
- **403 `no_domain`** (right password, no domain, not an Operator): a card "No domain yet": "Your
  password is right, but your account is not in any domain yet. An admin adds you to a domain. Then sign
  in again." and **Back to sign-in**. No session exists.
- **Too many tries (429):** "Too many tries. Wait a minute, then try again." The button stays disabled
  until then. **Service down (503 or no answer):** "We cannot reach sign-in right now. Try again in a minute."
- **Back on this device:** with "Remember my username", the next visit says "Welcome back", fills the
  field and focuses Password, with "Not ana? Use another username".
- **Forced password change** (`Session.mustChangePassword`, AC-ID-9): only
  `password-change-required.html` shows until the person saves: "Choose your own password", the
  password from the admin, a new password, **Save and continue** (`POST /identity/password`, then the
  default domain) and **Sign out**. `400 password_policy` shows the server's rule under the new password.
- **Operator with no domain** (session domain `*`, AC-ID-14) who opens the messenger sees "Your account
  has no domain" with **Open Mesail admin** (`domain-switch-states.html`).
- **Sign-out** ends the one Keycloak session, so every domain and every tab of that session signs out
  (`signed-out.html`). The "session ended" dialog keeps the draft (`session-expired.html`).
- Connection banners use one wording everywhere: "Reconnecting… New messages may be late." with
  the button **Try now**.
- **Sign in with a passkey** (section 17, `signin-passkey.html`): where the device supports passkeys, the
  form also has an "or" line and **Sign in with a passkey** under **Sign in**, and the username field offers
  saved passkeys in the browser's autofill. The admin app's form gets the same.

## 9. Settings and admin

- **Settings:** profile (the name is changed by an admin and is the same in every domain; the username
  never changes), notifications (push permission states, email summary delay, do-not-disturb schedule),
  email (send-from aliases, signature, remote images, Cc default), appearance (DuVay themes light,
  dark, auto, high contrast; density; text size; motion). Notifications and email settings are per
  domain; appearance is per device; the account is one for every domain.
- **Default domain** (`settings.html#default-domain`, `PUT /directory/me/default-domain`): a select of
  the person's domains, "Opens when you sign in with ana." Saved: "Saved. Signing in with ana now opens
  example.org." `404`: "You are no longer a member of lab.example.net. Choose another domain."
- **Account security** (SPEC AC-ID-7, AC-ID-8; `settings.html`, `settings-security.html`): the
  "Account and sign-in" card has a **Password** row ("Change password") and a **2-step sign-in** row
  (status from `GET /api/v1/identity/totp`: On or Off; "Turn on", or "Set up on a new phone" and
  "Turn off"). Each opens a dialog (full screen under 600 px).
  - Change password: current password, new password (Show toggles, no repeat field), 2-step code
    when on. One message for a wrong password or code; the server's rule text for `password_policy`.
    Success keeps this session and says other devices were signed out. It is the password for every domain.
  - Turn on: three steps (confirm password; scan the QR code or type the key, grouped by 4 with Copy;
    type the 6-digit code). "Set up on a new phone" also asks for the current code in step 1.
    No recovery codes: "If you lose your phone, ask your admin to reset 2-step sign-in."
  - Turn off: password and code, with the warning "Without it, anyone with your password can sign in."
  - **Passkeys** row (section 17, `settings-passkeys.html`): after the 2-step sign-in row. SLICES.md calls this
    card "Settings > Sign-in"; there is no separate Sign-in page.
- **Admin app** (`apps/web-admin`, its own shell: a rail of the domains the person administers plus a
  "Whole install" tile for Operators; a menu with **Install** (Domains, People; Operators only) and
  **the domain** (Domain, Members, Channels; its admins)). The server checks every role; the UI only
  hides entry points. Sign-in uses the same one-field form (`admin-signin.html`); a person who is
  neither an admin nor an Operator sees "You need admin rights". The older admin mockups
  (`admin-domains.html`, `admin-domain-*.html`, `admin-channels.html`, `admin-mail.html`) show the same
  pages inside the messenger shell; build them in the admin shell.
  - **Members** (`admin-members.html`, `admin-members-states.html`): find, **Add member**, a table of
    member (name and username), address, role (Admin tag) and a row menu: role in the domain (Member or
    Domain admin), account actions (Reset password, Remove 2-step code, Change name, Turn off account:
    they change the person in every domain), and Remove from the domain. Under 48rem of page width each
    row is a card. "Only you" and "no match" states offer Add member.
  - **Add member** (`admin-member-add.html`, AC-ID-12): step 1 asks only the username (the field shows
    `@example.com` after it) and "Make this person an admin". An existing person is added as they are:
    "eve already had a Mesail account ... Mesail did not ask her first." A new username answers
    `400 invalid_request` with field `displayName`; then step 2 asks a display name and a first password
    ("Give it to Dan in person or by phone"). Errors: prefix rules, a role name, `409 duplicate` (alias or
    channel address uses it), already in the list (checked in the browser), `password_policy`.
  - **Member actions** (`admin-member-actions.html`): remove (lists what happens: access ends within a
    minute, mail refused, messages and account stay, the username stays theirs), `409 last_admin`
    ("example.com needs an admin ... Make another member an admin first."), reset password (sets
    `mustChangePassword`, signs out everywhere), remove 2-step code, remove all passkeys (section 17),
    turn off, change name.
  - **Who may change an account** (AC-ID-13): an Operator, or an admin of **every** domain of the person.
    Today `MemberSummary` does not say whether that holds, so the menu always offers account actions and
    a `403` opens "You cannot change Eve Park's account: Eve Park is also a member of a domain you do not manage."
  - **People** (`admin-people.html`, Operators only): find by name or username, filter by state, each
    person's domains with an admin mark, state (Active, Turned off, Operator, Deleted), and actions;
    **Delete person** asks to type the username and says "The username dan is never given to anyone else."
  - **First admin of a domain** (`admin-domain-first-admin.html`, S11): after DNS, the domain page asks
    "Name the first admin of example.org" with the same username flow (`POST
    /directory/domains/{domainId}/members` with `admin: true`).
- **Mockups:** `settings*.html`, `admin-*.html`.

## 10. Accessibility checklist (AC-UI-2)

- Landmarks: rail `nav "Domains"`, sidebar `aside`, conversation `main`, thread `aside`; one `h1`.
- Skip links to the messages and to the composer.
- Timeline is `role="log"`; new messages are announced through one polite live region
  ("New message from Bo Lee in #releases"), batched, never per keystroke.
- Every icon-only button has an `aria-label`; shortcuts are exposed with `aria-keyshortcuts`.
- Menus use `role="menu"` with `menuitemradio` for the chip's delivery choice; dialogs trap focus,
  close on Esc and return focus to the opener.
- Focus ring: DuVay's 2 px ring in the primary colour, 2 px offset, on every control.
- Contrast: all text pairs meet 4.5:1 (3:1 for large text and control borders) in light, dark and
  high-contrast themes. Delivery, presence and status never rely on colour alone.
- Targets: 24 px minimum with a pointer, 44 px on touch screens (`pointer: coarse`).
- Motion: 120-200 ms; everything respects `prefers-reduced-motion`.
- Text never below 12 px; body 15 px at 1.5 line height.

## 11. DuVay web components and Tailwind

The user's rule (2026-11-10): "use DUVAY web components for rigidity and Tailwind for fluidity", and be
super responsive on every platform the frontend ships to.

- **Pinned versions:** DuVay **v26.35.01-1** (github `d31ma/DUVAY`, tag `v26.35.01-1`, commit `bc8b56a`);
  Tailwind CSS **4.3.5** (`tailwindcss` and `@tailwindcss/cli`). The TACHYON website already runs the same
  stack (DuVay `<w-*>` elements, Tailwind 4, Tac) and is the model for the build.
- **DuVay owns the look and every generic control.** Buttons, icon buttons, chips, avatars, badges, lists,
  menus, dialogs, bottom sheets, popovers, tooltips, snackbars, banners, alerts, empty states, skeletons,
  progress, tabs, text fields, selects, comboboxes, checkboxes, switches, radios, the 2-step code input,
  the command palette, cards, tables and the bottom navigation are DuVay `<w-*>` light-DOM web
  components. DuVay's tokens and themes are the source of truth for colour, type, spacing, radii, focus,
  motion and breakpoints; Mesail's `--ms-*` tokens cover only what DuVay has no name for (TOKENS.md).
- **Tailwind owns layout and fluid spacing:** the shell grid and how panes collapse, pane and card
  container queries, the composer rows, lists, gaps, padding, sizes, safe areas and the `short:` and
  `pointer-coarse:` variants. Tailwind's colours, fonts and radii are mapped onto DuVay tokens (`@theme
  inline`), its default palette is removed, and its preflight is not used (DuVay's reset is).
- **Mesail components** fill the gaps no DuVay element covers: the chat timeline and message row, the
  email card, the composer frame, the recipient "how" label (Chat / Email), the domain tile, presence
  shapes, the delivery line. They are Tac components built from DuVay elements, styled with tokens.
- **Cascade order:** reset, DuVay, Tailwind theme, Mesail components, Tailwind utilities. DuVay sits in a
  lower `@layer`, so a Tailwind utility always wins over a DuVay default.
- **Loading and building in Tac** (ADR 0004 section 8): F1's `scripts/vendor/duvay.sh` builds the app's
  only DuVay bundle from DuVay's `src/` (the elements listed in `scripts/vendor/duvay-components.txt`,
  which starts from these mockups' list; `base.js` first; Mesail's icon set registered as DuVay's `ms`
  set before any element renders), vendors it in `client/shared/vendor/duvay/` with SHA-256 pins in
  `tac.scriptSources` / `tac.styleSources`, and loads it as a module. Tailwind 4.3.5 runs as the pinned
  standalone CLI (no Node packages); F1's Go tool `tools/styles` writes each component's utilities into
  its `tac.css` and `--check` gates staleness, so no layout lives in a global file. The files in
  `mockups/build/` build the mockups only.
- **Sizes (measured):** DuVay JS subset of 73 elements 341 KB minified, 77 KB gzip, 60 KiB Brotli (the Tac
  per-route script budget is 200 KiB Brotli); DuVay CSS subset 262 KB, 37 KB gzip;
  Tailwind output for all 47 mockups 53 KB, 9.3 KB gzip. Never load DuVay's full bundle next to the
  subset (some elements have no `customElements.get` guard).
- **Rules for code:** never use `@event` or `on<name>` attributes on `<w-*>` elements (DuVay evaluates
  them with `new Function`, which the CSP forbids); bind with Tac's `on:` (`on:change`, `on:input`,
  `on:close`, `on:toggle`); DuVay's `update:modelValue` cannot be bound from Tac. Use `w-text-field`
  for text input. Never use DuVay's own utility classes (`h-100`, `w-sm-50`, ...): their names clash
  with Tailwind's. Mesail sets `w-theme` (default `auto`, following the system; the mockups open in
  `light`), `w-density` and `w-os` on `<html>` itself (DuVay's `duvay.js` is not loaded). For values
  that change often, set the element's property, not its attribute (a changed attribute re-renders it).
- **DuVay gaps found** while building the mockups (worked around in Mesail; worth reporting to DuVay
  once the user agrees). Slices must apply the same workarounds:

  | Element | Gap | Mesail workaround |
  | --- | --- | --- |
  | `w-chip` | Always a toggle `<button aria-pressed>`; Enter or Space toggles `selected`; host `aria-*` not passed on; the "×" is a `span role=button` that keyboard users cannot reach; `prepend-icon` prints text | Recipient chips are a Mesail component: a real `<button aria-haspopup="menu">` drawn with DuVay's chip classes plus a separate remove button; static labels use `.ms-tag`; icons are slotted `<w-icon>` |
  | `w-card` | Plain children land in a hidden slot after a re-render, so the card shows empty | Use the `w-card-*` parts, or a Tailwind container |
  | `w-text-field` | Does not pass `inputmode` or `aria-describedby` to the input; the clear button has `tabindex=-1`; long values cannot wrap | Patch upstream before F1 needs a number keypad; separate clear button; read-only `w-textarea auto-grow` for long values |
  | `w-btn` | No `type` and no `target` attributes; `aria-disabled` not passed on | Native `<a target>` for new tabs; `disabled` plus a visible reason; confirm form submit in F1 |
  | `w-alert` | Always `role="alert"`, so static notices announce on load | Static notices use a Tailwind box; `w-alert` only for real errors |
  | `w-banner` | Always a region named "Banner"; `icon` prints text; no email colour | `slot="prepend"` with `<w-icon>`, a `role="status"` line, `--w-banner-bg` / `--w-banner-fg` |
  | `w-snackbar` | Always `role="status"`; `prepend-icon` prints text | Error toasts add a `role="alert"` line; icon in the default slot |
  | `w-list` / `w-list-item` | Items get no `listitem` role; without `href` an item is a toggle button; it cannot be a `menuitemradio` | `role="listitem"`; nav items always have `href`; menus use plain buttons with menu roles |
  | `w-tabs` / `w-tab` | No roving tabindex, no `aria-controls`, `aria-label` not passed on; link tabs get no `aria-current` | Panels use `role="tabpanel"` + `aria-labelledby` |
  | `w-menu` | Always renders its own trigger and ignores `target` | Static frames in mockups; the menu component wraps its own trigger |
  | `w-hover-card` | Content is `role="tooltip"`, so it cannot hold buttons | Profile cards are a popover with `role="dialog"` |
  | `w-dialog` | Title is an `h3`; no `aria-describedby`; no phone sheet mode; `title` also shows a browser tooltip | `slot="title"`; Tailwind `max-sm:` rules make it a bottom sheet |
  | `w-select` | Its accessible name is the selected text, so the label is lost | Visible label plus `aria-labelledby` once patched |
  | `w-switch` | Announced as a checkbox | Accept, or patch `role="switch"` |
  | `w-btn-toggle`, `w-chip-group`, `w-avatar-group` | Group has no accessible name | Wrap in a labelled `role="group"` or use `w-radio-group` |
  | `w-otp-input` | Keeps one character per box when a password manager fills the code | Use `w-text-field` with `autocomplete="one-time-code"` |
  | `w-empty-state` | Title is a `<p>`; `icon` prints text | Slot a real heading and a `<w-icon>` |
  | `w-skeleton-loader` | Each loader is its own live region | Wrap in `aria-hidden`; one "Loading …" status line |
  | `w-table` | Nests two table roles | Semantic `<table>` styled with Tailwind |
  | `w-bottom-nav-item`, `w-bottom-navigation` | `icon` prints text; `grow` sizes the inner buttons, not the items | Slotted `<w-icon>`; Mesail CSS for equal widths |
  | `w-avatar` | Status dot labelled "warning badge"; only online, away, busy | Presence in `alt`; Mesail shapes for idle, do not disturb, offline |
  | `w-command-palette` | Items are text only (no avatars, tags, snippets) | Mesail palette list inside the same pattern |

## 12. Platforms and input

The frontend ships through Tac to the web, macOS (WKWebView), Windows (WebView2), Linux (WebKitGTK),
iOS (WKWebView) and Android (WebView). Engine floor: Safari 17.4, Chromium 118, WebKitGTK 2.44.

- **Platform skins:** DuVay's `w-os` (`macos`, `windows`, `linux`, `ios`, `android`) changes fonts,
  chrome heights, focus ring and motion, never colour or control size. Tac exposes no platform marker, so
  the shell sets `w-os` from the Tac target at start-up. Mockups take `?os=` to preview a skin.
- **Window chrome and safe areas:** Tac pages have no `viewport-fit=cover`; with
  `titleBar: "integrated"` Tac reports `--tac-titlebar-height`, `--tac-titlebar-leading-inset` and
  `--tac-titlebar-trailing-inset` (traffic lights on macOS, caption buttons on Windows, notch and
  status bar on iOS and Android). Every top bar uses the `safe-t` utility, side edges `safe-l` / `safe-r`,
  bottom bars `safe-b` (falling back to `env(safe-area-inset-*)` on the web). Top bars are drag regions
  (`data-tac-drag-region` in Tac; `drag-region` / `no-drag` in the mockups).
- **Pointer and touch:** hover affordances (message toolbar) appear only on devices that can hover, and
  always on keyboard focus; touch uses long-press sheets. Under `pointer: coarse` every target is at least
  44 px (DuVay forces compact density back to 44 px too). Keyboard-shortcut hints hide on touch.
- **Keyboard:** every action is reachable by keyboard; shortcuts show with `w-hotkey keys="cmd+k"`,
  which prints ⌘ on Apple platforms and Ctrl elsewhere.
- **Orientation:** layouts follow width, not orientation; landscape phones get the two-pane layout from
  600 px and the `short:` adjustments.
- **CSS features to avoid** (not in the engine floor): anchor positioning, container style queries,
  view transitions, `@starting-style`, `light-dark()`, relative colour syntax, `field-sizing`.

## 13. Design decisions made while building the mockups

- **Send keys.** In chat mode, Enter sends and Shift + Enter adds a line. In email mode (any email
  recipient), Enter adds a line and Ctrl or Cmd + Enter sends, because emails are longer. The hint
  under the composer always shows the current keys.
- **Opening a conversation marks it read.** The current sidebar row is never bold and has no count.
- **Internal recipients on the email's Cc** (S4): their name is on the Cc line so outside replies
  reach them; they get a mailbox copy only when their chip is set to *Email* or *Chat and email*.
  The composer has a switch to leave them off the Cc line.
- **Unverified senders.** Images from a sender that fails SPF, DKIM or DMARC stay hidden, and
  "Show images" is not offered; "Show original" is.
- **Appearance settings are per device;** all other settings are per domain.
- **Mockup-only helpers** (labelled state frames, `?theme=`, `?pane=`) are not part of the product.

## 14. Open questions

- **Q-UX-1 (resolved 2026-11-10): domain versus organization.** The user confirmed one chat space
  per domain: "internal" means the same domain, and chat never crosses domains, even inside one
  organization. Email crosses domains. An organization, if it stays, only groups domains for admin
  and billing. SLICES.md already resolves recipients as `member`, `channel`, `otherDomain` or
  `external`, which is what the chips show. mesail-arch is aligning SPEC.md and DOMAIN.md.
- **Q-UX-2: workspaces.** SLICES.md says a domain has exactly one messaging space, so these
  mockups show no workspace level, and "workspace admin" appears as "domain admin". SPEC.md still
  lists workspaces. If they come back, they become a second level under the domain (sidebar header
  menu), not a second rail.
- **Q-UX-3:** SPEC Q2 (offer *Chat and email* for a member). The design offers it in the chip menu
  only, never as a default.

Questions from the screen work (product owner to decide; none blocks the first slices):

- **Composer.** When Bo's chip is set to *Email*, does he still see the message in Mesail? SPEC S5
  says yes; then *Email* and *Chat and email* differ only in notifications (email only, or push and
  email). In a conversation with outside people, can a member post a chat-only note? The mockups
  assume yes, marked "Only people in example.com see this".
- **Delivery.** After an edit, offer "Send the change by email"? After a bounce, does "Send again"
  send a new email copy of the same message (mockup) or post a new message? Warn about repeated
  bounces once per address or once per message?
- **Sign-in.** Are drafts kept on the device or on the server, and does sign-out delete them? Who
  turns a domain off: the operator or the domain admin? (Resolved: people change their own password
  and 2-step code in Mesail's settings, AC-ID-7/8; admins manage members in the admin app through
  `directory`, never in the Keycloak console, ADR 0005.)
- **One login (for mesail-arch).** (a) A `MemberSummary.accountManaged` flag on the admin routes would
  let the members menu disable account actions up front instead of answering `403` (AC-ID-13). (b) After
  a refresh answers `401` because the open domain was removed, may the same `sessionKey` still switch to
  another domain (`POST /session/domain`)? The mockup tries it, then falls back to sign-in.
- **Offline first (slice OU): answered by mesail-arch on 2026-11-11 (SPEC on develop 86c1dda).**
  (a) Q13 is "first words" (decided); the access-lost notices show them. (b) A tab that is not kept keeps
  what it loaded and may write; writes wait in the tab's memory only ("Waiting to send. Closing this tab
  loses it.") and anything else says "Needs the network on this browser" (AC-OF-19). (c) "Remove the copy
  on this device" also turns off "Keep me signed in". (d) Files can be attached offline on a kept device,
  up to the domain's per-file limit and 200 MiB of queued files per device ("Attach this when you are
  online" beyond that); a refused upload goes to "Needs your attention". (e) The person chooses the space
  limit (300 MB, 1 GB, 2 GB, 5 GB), capped by the free space the device reports. (f) A mention of someone
  who left the channel is not a refusal: the message is sent, the mention shows as plain text.
- **Passkeys and the app lock (slice PU): answered by mesail-arch on 2026-11-12 (develop feb4b8d).**
  (a) **Rename:** CONTRACTS has `PATCH /api/v1/identity/passkeys/{passkeyId}` with `{label}` (SignedIn, no
  password; `200 Passkey`, `400 invalid_request` for a label outside 1 to 64 characters, `404 not_found`).
  (b) **The count of lost items** (AC-OF-20): PK6 keeps the number of waiting items (a count only, no content)
  outside the OS-locked tables, under the normal sealed key, and updates it with every queue change.
  (c) **An expired `setupKey`** (10 minutes): confirm answers `400 setup_expired`; every other failure stays
  `400 invalid_passkey`. (d) **Design choices confirmed:** without passkey support the Passkeys row still shows
  when the account has passkeys (Remove works, Add is hidden); the passkey button opens the default domain
  (AC-ID-16); the locked screen shows no counts; Rename needs no password.
- **Calls (slice CU): sent to mesail-arch on 2026-11-14; the mockups show the proposal in brackets.**
  (a) How long devices ring before `call.ring.end` [45 s]. (b) Do calls of two need the frame-transform hook? [no:
  a browser without it says "Group calls are not available in this browser" and calls of two work]. (c) Do not
  disturb [the card shows with no sound; no "Incoming call" push; the missed-call row still comes]. (d) More than
  one recorder at once [allowed; one banner lists every name]. (e) A declined call of two [the "Missed call" row].
  (f) A recording that fails part way [the part already written stays in the file]. (g) Switching domain during a
  call [asks "Leave the call and switch?"]. Also: SPEC section 6 still lists "End-to-end encryption" as out of
  scope, which contradicts ADR 0011, ADR 0012 and AC-CL-3.
- **Settings.** Does the job title come from Keycloak, like the name? Is "Always show images from
  people I have emailed" allowed (AC-IN-5 speaks of one sender at a time)? Is auto-reply out of the
  first release (the mockup shows it as "Later release")?
- **Search.** Do People hits match names and addresses only? Are recent searches kept on the device?
- **Admin.** Is the suppression list per domain (expected, now that the domain is the tenant)? Is mail to an
  archived channel's address refused (mockup) or accepted? Who can archive and unarchive? What
  happens to a turned-off domain (mockup: mail refused, sign-in blocked, messages kept)? When can
  the old DKIM record go (mockup: after 7 days)?

## 15. Offline first (ADR 0006, slice OU)

The server stays the source of truth. A **kept device** keeps a copy of recent messages and mail, works
offline (read, search, write), queues every change and sends it later. Mockups: `offline-messenger.html`,
`offline-status.html`, `offline-messages.html`, `attention.html`, `attention-states.html`,
`signin-kept.html`, `device-locked.html`, `access-ended.html`, `settings-device.html`. Styles:
`mockups/assets/screens/offline.css`. DuVay request D1 would replace the sync bar, the queue notes, the
attention cards and the space meter (COMPONENTS.md "Offline and sync").

- **Kept device** (`signin-kept.html`, AC-OF-1, Q8): the box "Keep me signed in on this device" sits
  under the code field with a hint. Browser: off, "Only on your own computer. Mesail then keeps your
  recent messages here and works offline." Ticked: "Mesail keeps your messages of the last 30 days on this computer, encrypted. Signing out removes
  them." Mesail app: on, "Recommended in the Mesail app. Turn it off on a
  device you share with others." Sends `keepSignedIn` in `SignInRequest`. Where the copy cannot be
  encrypted (AC-OF-18, Q15), a warning box under the box: "This device cannot encrypt Mesail's copy. Anyone
  who can read this device's files can read your recent messages." and "Untick the box to keep nothing
  here." The box stays hidden until the release gate of ADR 0006 section 9 opens (Q16). On a kept browser the app asks for persistent storage once, right after
  sign-in; Firefox shows its own prompt, and Mesail explains it first ("Firefox asks if Mesail may store
  data on this device. Choose Allow ..."; `signin-kept.html` 2b). If the person blocks it: "Firefox may clear
  them when space runs low; Mesail then loads them again when you are online. Items that wait to send could
  be lost then." with how to allow it later (2c, and a row in Settings > This device). When a device that had
  a plain copy can encrypt later, Mesail replaces the copy without asking (no screen; at most the sync bar's
  "Updating"). A kept device opens offline
  without the sign-in form. Without a copy and without network: "You are offline. Sign-in needs the network."
- **Sync bar** (`offline-status.html`): one full-width status bar at the very top of the app, above every
  pane and on every width, so the person always sees it in the same place. The bar itself never takes
  focus and is not clickable (DuVay `w-sync-status`, which never takes focus; decided 2026-11-11 after the
  DuVay trial 2). A **Details** button in the bar's actions slot (32 px with a mouse, 44 px on touch)
  opens the sync panel; "Try now", "Manage space" and "Review" sit in the same slot. The mockups
  `offline-status.html` and `offline-messenger.html` still draw the whole bar as one button: slice O4
  builds the Details button instead. It shows only when there is
  something to say, and it takes the top safe area (the pane headers under it do not add it again).
  32 px with a mouse, 44 px on touch and under 768 px. States and words:
  - Offline: "Offline · Updated 08:12", plus "4 waiting to send" ("4 waiting" under 600 px). Neutral
    colour: offline is normal, not an error.
  - Back online: "Updating · Getting changes since 08:12" with a 2 px progress line; then "Sending · 3 of 4";
    then "Up to date · 4 sent" in green, gone after 4 seconds (no slide with reduced motion).
  - "1 item needs your attention · 3 others sent" in amber, until the person opens the list.
  - Realtime down but network up: "Reconnecting · New messages may be late" and "Try now" (the existing
    wording of section 8).
  - Long offline (AC-OF-15): "Reloading recent messages". Before the 30-day lock: "Connect soon · Locks on 18 Nov".
  - Device full (`StoreWriteFailed`, AC-OF-13): "This device is full · New messages are not kept" and
    "Manage space" (opens Settings > This device). Waiting items and drafts are safe.
  - **Details** opens the **sync panel** (a dialog; a bottom sheet under 600 px): each domain with "Updated
    08:12" and its waiting count, domains without a copy marked "Not on this device. Needs the network.",
    a link to "Needs your attention", **Try now**, and "Space and offline settings". It says "Mesail sends
    only while it is open" (TACHYON T3 is not built, ADR 0006 section 9).
  - Screen readers hear one polite status per change ("Offline. Showing the copy on this device, updated
    08:12."; "Up to date. 4 sent."), never a stream of progress.
- **Waiting items** (`offline-messages.html`, AC-OF-4): a change shows at once. A queued message has a
  3 px amber edge on the left and a note under it; its text stays at full contrast, because it is the
  person's own writing. Notes: "Waiting to send", "Email to carol@partner.test: waiting to send", "Edit
  waiting to send" with **Undo edit**, "Deletes when you are back online" (text struck through) with
  **Undo**, and a reaction chip with a small clock. The time reads "Written 09:40" until it is sent; then
  the server time, with "Written offline at 09:40" (AC-OF-7). Back online, "Sending…" as today. The
  sidebar row of a conversation with waiting items shows a clock ("3 waiting to send" for screen readers).
- **Things not on this device** (AC-OF-3): a dashed panel at the top of the timeline, "Older messages are
  not on this device. This device keeps the last 30 days. Older messages load when you are back online."
  (14 days on a phone, or the person's choice.) Files that were not downloaded (over 2 MB on a computer;
  any file on a phone until opened, Q11) show "Not on this device · 14 MB · opens when you are online"; thread links "12 replies · 8 on this
  device". Online, the same place offers **Load older messages**. Things that need the network (create a channel, change notification settings) are disabled with
  "(needs the network)" in their label. Attaching works offline on a kept device: the file shows "2 MB ·
  uploads when you are online" and uploads first; over 200 MB of queued files: "Attach this when you are
  online".
- **Composer offline** (AC-OF-5): a line on top, "You are offline. What you send waits on this device and
  sends when you are back online." ("Offline: sends when you are back online." under 600 px). Members and
  channels come from the copy; any other address gets the chip "Checked when online". If the server
  later finds that an address needs email, the item goes to "Needs your attention" instead of being sent
  another way (AC-OF-6: never sent with a changed choice).
- **A tab that is not kept** (AC-OF-19; `offline-status.html` 7b, `offline-messages.html` 11c): the bar
  says "Offline · This browser keeps nothing" and "1 waiting in this tab"; a written item says "Waiting to
  send. Closing this tab loses it."; search, files not loaded and other conversations say "Needs the
  network on this browser".
- **Search offline** (AC-OF-12): "Search this device" in the sidebar; a line "Searching this device: the last 30 days of example.com. Older messages need the network."; each hit has the tag "On this device".
  No hit: "No results for … on this device", and a hint to search again online.
- **Needs your attention** (`attention.html`, `attention-states.html`, AC-OF-6, AC-OF-8, S16): a row under
  Activity with a red count; a page with one card per item, newest first, for the current domain, and a
  line for other domains ("1 more item needs your attention in example.org. Open example.org"). A card:
  where and when it was written, the reason as a heading, one sentence on what happened, the person's
  text (3 lines at most), and buttons that depend on the reason. **Discard** is always last and asks first.
  - Recipient changed: "Bo Lee is no longer in example.com" → **Send by email**, Edit, Discard.
  - Edit conflict: "Your edit was not saved" with "Now on the server" and "Your edit (draft)" side by side
    (stacked in a narrow card) → **Use my edit**, Keep the newer text, Edit.
  - Too large, attachment refused: Edit, Discard. Conversation gone or posting not allowed: **Copy text**,
    Discard. Any other refusal: **Retry**, Edit, Discard.
  - Dropped changes (a refused send's later edits and reactions, or changes to a message deleted
    elsewhere): an info card or a system line in the timeline, "Your edit and 1 reaction were not saved".
- **Domains** (AC-OF-11): the domain menu offline says "Offline: domains on this device still open." Each row
  shows "On this device · updated 08:12" and its waiting count; a domain without a copy is disabled with
  "Not on this device · needs the network" (the rail dims its tile). A switch offline opens that domain's
  copy at once ("Now in example.org. Offline, updated 07:55."); online, the tab still calls
  `POST /identity/session/domain`.
- **Locked copy** (`device-locked.html`, AC-OF-14, Q10): after 30 days without an update, or after a plain
  `401`, the app shows "Your messages on this device are locked" (or "Sign in again") with the person's
  name, password and code, the waiting count ("3 items you wrote offline wait to send. They send after you
  sign in."), and "Not ana? Use another username", which asks first: "Remove Ana Ruiz's messages from this
  device?" (all copies, drafts and unsent items).
- **Access ended** (`access-ended.html`, AC-OF-9, AC-OF-10, Q13): `403 access_revoked` shows "Your session
  was ended" and "Mesail removed your messages from this device. 2 items you wrote offline were not sent."
  Removal from one domain shows a one-time dialog, "You no longer have access to example.org ... 2 items
  you wrote offline for example.org were not sent:" followed by the first words of each item, one per line
  (Q13 decided; "This is the last time Mesail shows them."). 
  Sign-out with waiting items warns first (offline: "Sign out and remove"; online: "Wait, then sign out").
  After another person signs in, a notice says the last person's copy was removed, without a name.
- **Settings: this device** (`settings-device.html`, AC-OF-13, Q9, Q11): a new settings section "This
  device" (tab and side list on every settings page). Cards (the space limit is capped by the free space the device reports): Keep me signed in (switch, since when, and
  "Mesail encrypts the copy with a key that only this device holds", AC-OF-14); Space used (real numbers from `StorageSpace.estimate()`: "412 MB of 1 GB used", "38 GB free on this
  computer"; a bar split by domain, with a list of each domain's size and update time, so colour is never
  the only cue); What to keep (7, 14, 30 or 90 days as a segmented
  choice, space limit, "Download files ahead of time" on a computer or "on Wi-Fi" on a phone); Remove
  ("Remove the copy", "Sign out everywhere"). Variants: Firefox with storage not kept, a device that cannot encrypt (the AC-OF-18 warning
  with "Remove the copy"), a full device, a phone (14 days, files on Wi-Fi), and a browser without a kept
  session (how to turn it on). There is no "first build" with smaller limits any more (Q16: the first
  offline release waits for sealed tables and has the full window and files).
- **Sizes:** every new target follows TOKENS.md "Control sizes"; the bar, chips, list rows, tabs and
  recipient input reach 44 px on touch and under 768 px. Checked in headless Chromium at 320, 375, 768,
  1024, 1440 and 2560 px, light and dark (fine and coarse pointer at 768 and 1024).

## 16. Notifications when Mesail is closed (browser push, SPEC AC-PN-3, slice N2)

Mockups: `push-states.html` (every state), `settings-notifications.html` (the row in place). Server side:
CONTRACTS "notify" (`GET /notify/push-key`, `POST` and `DELETE /notify/push-subscriptions`, the payload
`{title, body, path, tag}` of TACHYON ADR 0118).

- **The row** (Settings > Notifications, card "Notifications when Mesail is closed", "For this browser, in
  every domain you open here."): the switch "Notify me in this browser when Mesail is closed", off by
  default, with "Turn this on only on your own device. A notification shows who wrote and where, never the
  message." It is per browser, not per domain: while it is on, each domain the tab opens registers the
  same browser subscription once.
- **Mesail never asks on page load.** The browser's own question appears only right after the person turns
  the switch on, or presses "Turn on" in the suggestion. While it is open the row says "Your browser now
  asks if Mesail may show notifications. Choose Allow." (the switch disabled, a spinner). Results:
  - Allow: "On for this browser in example.com and example.org. Other domains join when you open them here."
  - The question closed without a choice: the switch goes back off, "Nothing changed. Turn the switch on to
    be asked again."
  - Block (now or earlier): the switch is disabled, the hint says "Blocked in this browser's settings.", and
    a warning box explains how to allow it (the icon left of the address, Notifications, Allow) with
    **Check again**.
  - Turned off by the person: "Off. This browser no longer gets notifications when Mesail is closed. Your
    other devices keep their own setting." (`DELETE /notify/push-subscriptions` in each registered domain.)
- **Hidden** where the browser has no push: in the Mesail apps (their push comes later, TACHYON T4b), and in
  Safari on iPhone or iPad unless Mesail runs from the Home Screen. Design decision: on iPhone and iPad the
  row is replaced by one hint line, "To get notifications when Mesail is closed on iPhone or iPad, add Mesail
  to your Home Screen ...", because that is the only way to get them there.
- **The one-time suggestion**: shown once per browser, after the first notification arrives while Mesail is
  open, never on page load. A small card under the sidebar list (above the bottom navigation on phones):
  "Get notified when Mesail is closed?" with the own-device advice, **Turn on** and **Not now** (the close
  button counts as Not now). Either answer hides it for good on this browser. After "Turn on" the card says
  "Your browser now asks"; then one status line for 4 seconds: "Notifications are on for this browser.",
  "Not turned on. You can turn it on later in Settings > Notifications." or "Blocked in this browser's
  settings." with "How to allow".
- **The notification** (drawn by the system; its look differs per OS): title = the sender's name; body = what
  and where, never message text: "mentioned you in #design (example.com)", "sent you a message
  (example.org)", "replied in a thread (example.com)". A newer push for the same conversation replaces the
  older one (`tag`). Nothing comes during Do not disturb or while the person is active in Mesail.
- **The click** opens the conversation at the first unread message, in its domain. When the tab shows
  another domain it switches first (`POST /identity/session/domain`) and says "Now in example.org, from a
  notification."; with no Mesail tab open a new tab opens there. Lost access shows the existing "You no
  longer have access to example.org"; signed out shows the sign-in form and opens the conversation after
  it; a conversation the person can no longer read says "This conversation is no longer available to you."
- **Where pushes stop**: sign-out (the confirmation says "This browser also stops getting notifications
  from Mesail."; `signed-out.html` adds "Notifications in this browser are off."), every wipe of the device
  copy (the "Remove the copy" confirmation says so), removal from a domain (that domain's registration is
  deleted by the server; the existing notice covers it) and `403 access_revoked` (the "Your session was
  ended" screen covers it).
- **Sizes and checks:** the row uses the shared control scale; every new target is at least 44 px on touch
  and under 768 px. Checked in headless Chromium at 320, 375, 768, 1024, 1440 and 2560 px, light and dark.

## 17. Passkeys and the app lock (ADR 0008, slice PU)

Mockups: `signin-passkey.html` (every sign-in state), `settings-passkeys.html` (the list, add, rename,
remove), `settings.html#passkeys` (the row in place), `admin-member-actions.html` 10-11 and the person menus
in `admin-members.html` and `admin-people.html` (remove all), `app-lock.html` (the locked screen and the
setting), `settings-device.html#unlock` (the setting in place). Server side: CONTRACTS "identity"
(passkeys) and `DELETE /directory/people/{personId}/passkeys`. Client side: TACHYON `Passkeys` (T13, TACHYON
ADR 0126; T14 in the apps) and `StorageLock` (T15, T15b). A passkey is an **extra** way to sign in (Q21):
the password and 2-step code always keep working, and every screen says so.

**Where it shows.** `Passkeys.check()` answers `PasskeySupport { platform, conditional }`. Until it answers,
and when `platform` is false (an old browser, the Linux app, the other apps until T14), nothing about
passkeys shows: the sign-in form is exactly `signin.html`. The only exception is the Passkeys row in
settings (below). The app lock shows only in the macOS, iOS, Windows and Android apps, on a device that
keeps a copy; browsers and the Linux app never show it (`StorageUnlockUnavailable`).

### 17.1 Sign in with a passkey (AC-ID-16, S18)

- **Place:** under **Sign in**, an "or" line, then **Sign in with a passkey** (outlined, 40 px, 44 px on touch,
  full width, fingerprint icon) with the hint "Touch ID, Face ID, Windows Hello or your device PIN. No password
  needed." The password path stays first and largest, because every account has a password. "Keep me signed
  in" and "Remember my username" sit above both buttons and apply to both (`keepSignedIn` in `PasskeySignIn`).
- **The challenge is fetched before the click.** Safari refuses `get` after an await, and TACHYON ADR 0126
  allows `get` only inside a click or key handler. So the page calls `POST /identity/passkeys/challenge` when
  the form opens (and again after 2 minutes, or after each use), and the click handler calls
  `Passkeys.get(optionsJson)` at once.
- **Autofill:** with `conditional: true` the username field gets `autocomplete="username webauthn"` and the
  page calls `Passkeys.offer(optionsJson)` when the form opens. The browser lists saved passkeys in the
  field's own dropdown; choosing one signs in like the button. Its hint then says "Choose your passkey in the
  list, or type your username." Any typed sign-in or a click on the button stops the offer first.
- **Waiting:** the button shows a spinner and is disabled; the hint says "Your device asks for Touch ID, Face
  ID, Windows Hello or its PIN. Follow what it shows."; a text button **Use your password instead** calls
  `Passkeys.cancel()`. Typing a password and pressing **Sign in** also cancels the passkey call first.
- **After the device answers:** "Signing in…" while `POST /identity/session/passkey` runs. Success opens the
  default domain (no 2-step code is asked). A full address in the field does not change that; to open
  another domain, switch after sign-in. With "Remember my username", the username from the `Session` is kept.
- **Lines under the button** (one at a time; the newest replaces the older; focus stays on the button):

  | Cause | Kind | Words | Button after |
  | --- | --- | --- | --- |
  | `PasskeyFailed cancelled` (closed, timed out, nothing chosen) | info, polite | "No passkey was used. Try again, or sign in with your password." | shown |
  | `401` from the server (unknown, removed or wrong passkey; counts against the lockout) | error | "This passkey did not work. It may have been removed from your account. Try another passkey, or sign in with your password and code." | shown |
  | `busy` | warning | "Your device is already asking for a passkey. Finish or close that window, then try again." | shown |
  | `refused` (relying party does not match the page: an install set-up fault) | error | "Passkeys do not work on this address of Mesail. Sign in with your password. Tell your admin, so they can fix it." | hidden |
  | `unsupported` (on the click, though `check` said yes) | info | "This device cannot use passkeys here. Sign in with your password." | hidden |
  | `invalid` or `failed` | error | "Sign-in with a passkey did not work. Try again, or sign in with your password." | shown |
  | `429` | warning | "Too many tries. Wait a minute, then try again." | disabled with **Sign in** for that minute |
  | `503` or no answer | error | "We cannot reach sign-in right now. Try again in a minute." | shown |
  | `403 no_domain`, `403 not_a_member` | the same boxes as `signin.html` | | |

  `exists` cannot happen on sign-in; it belongs to "Add a passkey". The `message` of `PasskeyFailed` is never
  shown. Screen readers hear the line once (`role="status"` for info, `role="alert"` for the others).
- **A locked device copy** (`device-locked.html`) offers the same button. If the passkey belongs to another
  person, the "Remove Ana Ruiz's messages from this device?" dialog (state 4) shows before anything opens.

### 17.2 Settings: Passkeys (AC-ID-15, AC-ID-17, AC-ID-18)

- **The row** sits in Profile and account > Account and sign-in, after 2-step sign-in: fingerprint tile,
  "Passkeys", a "2 of 10" tag, the help "Sign in with Touch ID, Face ID, Windows Hello or your device PIN
  instead of your password. Your password and 2-step code keep working.", and **Add a passkey**. Under it, one
  line per passkey from `GET /identity/passkeys`, most recently used first: the name, then "Added 12 Nov 2026
  · Last used today, 08:55" (or "Not used yet"), and two text buttons **Rename** and **Remove** (their
  accessible names add the passkey's name). Dates use the person's locale; "today" and "yesterday" in words.
  Mesail cannot tell which passkey lives on this device, so no row says "this device".
- **States:** loading (skeleton, one status line), "Could not load your passkeys." with **Try again**, none
  yet ("You have no passkeys yet."), 10 of 10 (**Add** disabled with "You have 10 passkeys, the most you can
  have. Remove one to add another.").
- **No passkey support here:** **Add** is hidden. When the account has passkeys, the row and its list stay,
  with "This device cannot make or use passkeys. You can still rename or remove them here, for example when you
  lose a phone." (S18: removing a lost phone's passkey must work from any device). With no passkeys, the
  row is hidden.
- **Add a passkey** (a dialog; full screen under 600 px):
  1. "Step 1 of 2 · Confirm it is you": password, and the 2-step code when 2-step sign-in is on
     (`POST /identity/passkeys/setup`). `401`: "The password or 2-step code is not right. Check them and try
     again." (both fields cleared, focus on Password). `409 too_many`: "You already have 10 passkeys ..." with
     **OK**. `429` as elsewhere.
  2. "Step 2 of 2 · Make the passkey": **Name** (prefilled with the device kind, "MacBook", "iPhone", "Windows
     PC", "Android phone"; 1 to 64 characters), the line "Your device then asks for Touch ID, Face ID, Windows
     Hello or its PIN. Mesail never sees your fingerprint or face. It gets only a key that proves it is you.",
     and **Create passkey** (48 px), whose click calls `Passkeys.create(optionsJson)` directly. Then "Waiting
     for your device…", then "Saving…" (`POST /identity/passkeys`).
  - Lines in step 2: `cancelled` "No passkey was made. Choose Create passkey when you are ready."; `exists`
    "This device already has a passkey for your account. Use it to sign in. To make a new one here, remove
    the old one from the list first."; `busy` as on sign-in; `refused` "Passkeys do not work on this address of
    Mesail. Tell your admin, so they can fix it."; `unsupported` "This device cannot make passkeys here. Add
    one on another device, such as your phone."; `invalid` or `failed` "Something went wrong, so no passkey was
    made. Try again."; `400 invalid_passkey` "Mesail could not save this passkey. Try again. If your device
    now shows a Mesail passkey that is not in this list, delete it in the device's settings."; `400 setup_expired`
    (more than 10 minutes since step 1): back to step 1 with "This took too long, so Mesail asks again."
  - Done: the dialog closes, the new row shows, and a snackbar: "Passkey "MacBook" added. Next time, choose
    Sign in with a passkey."
- **Rename** (a small dialog): **Name** with a counter, **Save**. Empty: "Type a name, like iPhone or Work
  phone." `404`: "This passkey was removed, maybe on another device. The list is now up to date." No password:
  a label is not a secret (`PATCH /identity/passkeys/{passkeyId}`; a `400 invalid_request` shows under the
  field like the empty-name line).
- **Remove** (a dialog, "Remove the passkey "iPhone"?"): "You can no longer sign in with this passkey.", a
  warning box **"Your other devices will be signed out"**: "All your other devices will be signed out, in every
  domain. This device stays signed in. A signed-out device deletes its copy of your messages the next time it
  connects.", then password and code, and the tip "If you still have the device, also delete the passkey
  there, for example in System Settings > Passwords on a Mac." The danger button says what happens: **Remove
  and sign out other devices**. `200 Session` replaces this tab's tokens without a visible step. Done: the row
  goes, snackbar "Passkey "iPhone" removed. Your other devices were signed out. You are still signed in
  here." `401`, `429` and `404` as above.

### 17.3 Admin: Remove all passkeys (AC-ID-13, AC-ID-18)

- In the person menu (Members and People), in "Account of Dan (all domains)", after **Remove 2-step code…**:
  **Remove all passkeys…** with "For a lost device; signs out everywhere". It shows for every person, because
  `MemberSummary` does not say whether the person has passkeys; the call answers `204` either way.
- The dialog "Remove all of Dan Example's passkeys?": when to use it, that Dan can no longer sign in with any
  passkey but still with the password and 2-step code and can add new ones, and a warning box "Dan will be
  signed out on every device, in every domain, within a minute. Devices that are signed out delete their copy
  of Dan's messages." **Remove all passkeys** (danger). Done: "Passkeys removed" with "Dan has no passkeys now
  and is signed out everywhere." `403` shows the existing "You cannot change Eve Park's account" (state 6).

### 17.4 The app lock: Unlock with Touch ID (AC-OF-20, Q22)

- **The setting** (Settings > This device, inside "Keep me signed in on this device", after the switch): a
  switch **Unlock with Touch ID**, off by default, with "Mesail asks for Touch ID when you open it, after 5
  minutes in the background, and when this Mac locks. Your Mac password works too." It shows only while the
  device is kept signed in. Names per platform: macOS "Unlock with Touch ID" (no Touch ID: "Unlock with your
  Mac password"); iOS and iPadOS "Unlock with Face ID" or "Touch ID"; Windows "Unlock with Windows Hello";
  Android "Unlock with fingerprint or face" (no biometric: "Unlock with your screen lock").
- **Turning it on or off** needs the same check: the switch is disabled with "Confirm with Touch ID…" while the
  system prompt is open (`StorageLock.unlock()`, then `StorageLock.require(true|false)`). On: "On. Mesail is
  locked when you open it, after 5 minutes in the background, and when this Mac locks." Not confirmed when
  turning off: "Still on. Turning it off needs Touch ID or your Mac password." `lockedOut`: "Touch ID is locked
  after too many tries. Unlock your Mac with its password, then try again." `notEnrolled`: the switch is
  disabled with "Set up Touch ID or a password for this Mac first, in System Settings > Touch ID & Password."
- **The locked screen** covers the whole window (and so the app switcher's snapshot): a lock tile, **"Mesail
  is locked"**, "Unlock to see your messages.", the person's name and username, **Unlock with Touch ID** (48 px),
  "Sending, updates and new messages wait until you unlock.", and a text button **Cannot unlock? Sign out**.
  It shows no message, no conversation name, no domain and no count: while locked the app reads nothing from
  the copy (the queue and drafts are locked too). On open, the system prompt opens by itself once; after
  that only the button asks. After a relock: "Locked after 5 minutes in the background." No sync bar shows.
- **Lines:** asking (spinner, "Waiting for Touch ID…"); `cancelled` "Not unlocked. Choose Unlock with Touch ID to
  try again."; `failed` "Touch ID did not work. Try again, or choose Use Password in the window that opens.";
  `lockedOut` (button disabled) "Touch ID is locked after too many tries. Unlock your Mac with its password, then
  try again." with **Try again**. The OS draws its own prompt; its reason text comes from
  `tac.storage.unlock.reason`: "unlock your messages".
- **`keyLost`** (the passcode was removed or the enrolled fingers or faces changed): a dialog "Touch ID changed
  on this Mac": "The fingerprints or the password of this Mac changed, so Mesail can no longer open its copy of
  your messages. Mesail deleted that copy and loads your messages again from the server.", then "**3 items** you
  wrote offline and had not sent were lost with it." (left out when 0), "You are offline. Your messages load when
  you are back online." when offline, and "Unlock with Touch ID stays on. From now on it uses the fingerprints this
  Mac has now." **Continue**.
- **Cannot unlock? Sign out:** "Sign out and remove your messages from this Mac?" with "Mesail cannot open the
  copy while it is locked, so it cannot tell you what was not sent. Anything you wrote offline and did not send is
  lost." **Sign out and remove** (danger).
- **Both locks:** when the copy is also locked after 30 days (`device-locked.html`), the OS unlock comes first
  (Mesail needs the key to read the copy), then the sign-in.
- **Sizes and checks:** no new token; one new icon, `fingerprint`, in `mockups/build/mesail-icons.js` (F1 adds
  it to the app's icon set). Every new target is at least 44 px on touch and under 768 px. Checked in headless
  Chromium at 320, 375, 768, 1024, 1440 and 2560 px, light and dark, coarse pointer at 375, 768 and 1024: no
  sideways scroll, no small targets.

## 18. Audio and video calls (ADR 0012, SPEC AC-CL, slice CU)

Mockups: `call-view.html` (a group call in place, full window, responsive), `call-buttons.html` (where calls
start), `call-incoming.html` (ringing and the caller's side), `call-layouts.html` (two people, groups, screen
sharing, phones, the call bar), `call-controls.html`, `call-recording.html`, `call-history.html`,
`call-states.html` (permissions, not supported, connection problems). Server side: CONTRACTS "Signalling and
calls" (`call.create`, `call.join`, `call.ring`, `call.ring.end`, `call.leave`, `signal` of kind `call`) and
"calls" (`409 call_full`). Client side: TACHYON `Peers` (P1), `Media` (P5), `Recorder` (P7); the call client is
slice G12.

**What a call is.** A call belongs to one chat conversation of one domain: a direct conversation, a group or a
channel, all with at most 50 members (ADR 0011). Two people go device to device; three or more go through the
`calls` server, which cannot read the media. Every call is end-to-end encrypted, and the call view says so in its
header ("End-to-end encrypted", lock icon). Email conversations have no calls.

| Rule | Screens and states |
| --- | --- |
| AC-CL-1 Starting | 18.1; `call-buttons.html` B1-B7, `call-incoming.html` C1-C3, `call-history.html` H1, H3 (Join) |
| AC-CL-2 Size (50, 9 videos, speaker first) | 18.3; `call-view.html`, `call-layouts.html` V3-V5, V9; `call-states.html` P13 (full) |
| AC-CL-3 Private | header "End-to-end encrypted" in every call view; `call-states.html` P14 (removed from the conversation) |
| AC-CL-4 Ringing, push, missed | 18.2; `call-incoming.html` R1-R12; `call-history.html` H1-H3, H5-H6 |
| AC-CL-5 History rows | 18.6; `call-history.html` H1-H7 |
| AC-CL-6 Controls, last one out ends it | 18.4; `call-controls.html` K1-K11; `call-layouts.html` V12-V13 |
| AC-CL-7 Screen sharing on computers | 18.3; `call-layouts.html` V6-V8, V10; `call-states.html` P5 |
| AC-CL-8 Recording with the banner | 18.5; `call-recording.html` W1-W13; `call-incoming.html` R3; `call-view.html` |
| AC-CL-9 Not supported | 18.1, 18.7; `call-buttons.html` B9-B10; `call-incoming.html` R10; `call-states.html` P7; `call-controls.html` K5 (recording) |

### 18.1 Where calls start (AC-CL-1, AC-CL-9)

- **Two buttons in the conversation header**, before Details: **Audio call** (`phone`) and **Video call**
  (`video`). Their accessible names say who is called ("Video call Bo Lee", "Video call everyone in #design").
  Phones keep both as 44 px icon buttons.
- **A call of two starts at once.** For three or more members, a small popover asks first: "Start a video call
  in #design?" with "Everyone in #design is rung: 24 people. Anyone in #design can join while the call lasts."
  and **Start video call** / **Cancel** (design choice: ringing 50 people by a slip of the mouse is worse than
  one extra click).
- **A running call** replaces the two buttons with a green **Join** ("Join the call in #design, 3 people, 04:12");
  when this device is already in it, **Return to call**. A call row in the timeline also has **Join** (18.6).
- **One call at a time.** Calling while in another call asks "Leave your call in #design?" with **Stay in
  #design** and **Leave and call Bo Lee**.
- **Email conversations:** no buttons at all (nothing to explain; email contacts cannot be called).
- **Not supported (AC-CL-9):** where TACHYON answers "unsupported" (the Linux app at first), the buttons stay in
  place, dimmed, focusable, with names that end "Calls are not available on this device". Pressing one opens a
  popover "Calls are not available on this device" with where calls do work ("The Mesail app for Linux cannot
  make calls yet. Use Mesail in a browser on this computer, or on your phone."). A partial case: a browser that
  cannot encrypt group calls (no encoded-transform hook) says "Group calls are not available in this browser"
  on group buttons, and calls of two work (waiting for mesail-arch, section 14).
- **Offline:** the buttons stay; pressing one shows "You are offline. Calls need the network."

### 18.2 Ringing (AC-CL-4)

- **Computers:** a dark card at the top right of the window, over everything, `role="alertdialog"` that does
  **not** take focus (the person may be typing). Screen readers hear once, assertively: "Bo Lee is calling,
  video call. Press Control Shift A to accept, Control Shift D to decline." F6 moves focus into the card.
  Contents: the picture with a pulsing ring (still with reduced motion), the name, "Video call" or "Audio call",
  and the domain; for a group: "#design", "Chen Wu is calling · video call", "3 people are in the call". Buttons
  (56 px round): **Decline**, **Audio only** (video calls only), **Accept**. Esc does not decline.
- **A recording already runs:** the card shows the red "Recording by Chen Wu" line before the person joins.
- **Phones:** the whole screen, the caller large, **Decline** and **Accept**, and a text button **Answer without
  video**. A real phone call screen on the lock screen comes later (TACHYON P6); until then the push opens the app.
- **Sound:** a ring for up to 45 seconds (ring time to confirm with mesail-arch). Do not disturb: the card shows
  with "No sound: Do not disturb is on", and no push goes to other devices.
- **While in another call:** a compact card with **Decline** and **Leave and answer**.
- **Ends:** answered or declined on another of the person's devices: the card closes with "Answered on your other
  device."; nobody answered or the caller stopped: a status "Missed call from Bo Lee" with **Call back**, and the
  chat gets the missed-call row.
- **Devices that cannot call** are told, not rung: "Bo Lee is calling. Calls are not available on this device.
  Answer on your phone or in a browser." with **Dismiss**.
- **Push (no names):** "Incoming call" / "Open Mesail to answer." and "Missed call" / "Open Mesail to see it."
  Opening it shows "Opening the call…" while the app decrypts the invitation, then the ringing card; a call that
  ended meanwhile says "This call has ended." with **Call back**.
- **The caller's side:** a call of two shows "Calling Bo Lee…" and "Ringing" with the caller's own camera small,
  and **Cancel** in place of Leave. No answer and a decline use the same words, "Bo Lee did not answer", with
  **Call again** and **Close** (a decline is not shown as a refusal). A group caller is in the call at once:
  "Waiting for others to join" and "23 people are being rung. Anyone in #design can join."

### 18.3 The call view (AC-CL-2, AC-CL-7)

- **The stage is dark in every theme** (`--ms-call-*` tokens), like a video player, so video and shared screens
  look the same in light and dark mode. The view covers the whole window; the back arrow returns to the
  conversation while the call goes on (18.5). Header: conversation name, "25 people · example.com", "End-to-end
  encrypted" with the lock (the words hide under 600 px, the lock stays), and the call time.
- **Call of two, video:** the other person fills the stage; your own picture is small at the bottom right.
  Audio: both pictures in two tiles.
- **Groups:** a grid of at most 9 tiles. The person speaking is first (top left); the others follow by who spoke
  last (the `calls` server forwards the 9 most recent speakers). You are one of the tiles ("You"). When more
  people are in the call than fit, the last tile is "+17 more", which opens People. Columns follow the stage's own
  width: 1 or 2 columns up to 30rem, 2 up to 48rem, 3 from 48rem. A tile does not move for someone who speaks
  under 2 seconds, so the grid stays calm.
- **Phones (stage under 30rem):** 4 taller tiles (3:4): the speaker, one more person, you, "+22 more".
- **Tiles:** video, or the person's picture when the camera is off. A name label at the bottom left. The person
  speaking has a 3 px ring (`--ms-call-speaking`) **and** a sound-lines icon; muted people a slashed microphone;
  a weak connection a "Weak connection" badge. Each tile's accessible name says the same ("Bo Lee, muted").
- **People** (button with the count; docked at the right from 1280 px, a sheet below): "In the call (25)" with
  microphone state and notes ("Speaking · recording", "Sharing their screen"), then "Not in the call (2)" with
  **Ring again** per person.
- **Screen sharing (computers only, one person at a time):** the shared screen takes the large area and 4 speakers
  sit beside it (below it under 48rem); a note says "Ana Ruiz is sharing their screen". The sharer sees "You are
  sharing your whole screen. Everyone in the call sees it." with **Stop sharing**, and a tip to close private
  windows. While someone shares, Share screen says "Dara Okafor is sharing their screen. One person shares at a
  time." Phones see the shared screen across the width ("turn the phone for a bigger picture") and have no Share
  button. The system's own picker chooses what to share.

### 18.4 Controls (AC-CL-6, AC-CL-7)

- **The bar:** Mute, Camera off, Share screen (computers), People, More, Leave. Phones: Mute, Camera off,
  Speaker (loudspeaker on or off), People, More, Leave. Each is a real `<button>` with a caption under a 48 px
  face; the caption is the accessible name. Toggles use `aria-pressed`: pressed means OFF (muted, camera off), drawn
  as an inverted face **and** a slashed icon. Leave is red and wider. Under 25rem of stage the captions hide (names
  stay for screen readers) and faces shrink to the 44 px touch floor so all six fit at 320 px.
- **More:** Choose devices…, Record this call… ("Saved only on this device"), Keyboard shortcuts. Where recording
  is not possible, the item is disabled with "Recording is not available on this device".
- **Choose devices** (a dialog; a bottom sheet on phones): Microphone with a live level meter ("Speak to test it"),
  Camera with a preview, Speaker with **Play a test sound**; "Echo cancellation and noise reduction are on.
  Changes apply at once, in this call and the next ones on this device." Where the system decides the sound
  output (phones), the Speaker list is replaced by "Sound plays where your phone sends it. To use a headset or the
  car, choose it in the phone's own sound controls."
- **Device changes:** a status line, e.g. "AirPods Pro connected. Sound and microphone now use them." or "Your
  USB microphone was unplugged. Mesail now uses MacBook Pro Microphone."
- **Talking while muted:** "You are muted. Press Control Shift M, or Mute, to talk." for 4 seconds, at most once a
  minute (detected on the device; nothing is sent).
- **Leave** never asks, except while you record (18.5). In a group the call goes on ("You left the call. It goes
  on with 24 people." with **Rejoin**); the others hear a short tone and see "Bo Lee left" for 3 seconds. There is no
  "end for everyone": the call ends when the last person leaves ("Call ended · 12 min").
- **Shortcuts** (Cmd instead of Control on Apple; shown in Keyboard shortcuts and as `aria-keyshortcuts`):
  Control+Shift+M mute, +O camera, +E share, +H leave, +A accept with video, +S accept with audio only, +D decline;
  F6 moves between the stage, the bar and the people list.
- **The call bar outside the call view:** when the person opens another conversation, a dark bar sits on top of
  the main pane: "#design · 25 people · 12:04", Mute, Return to call, Leave (44 px on touch).

### 18.5 Recording on the device (AC-CL-8)

1. **More > Record this call…** opens "Record this call on this device?": the file is saved only on this device in
   a file the person chooses; it holds everyone's sound and the main picture (the speaker, or a shared screen);
   "Everyone in the call sees "Recording by Ana Ruiz" while it runs, people who join later too. Ask the others
   first."; "No server gets the recording. Mesail cannot stop other tools from recording a call." **Choose where
   to save…** / **Cancel**.
2. The system's save dialog (suggested name `design-call-2026-11-14-1004.webm`). Cancelled: "Not recording. No file
   was chosen."
3. "Telling everyone in the call that you will record…" while the notice goes to every device in the call
   (`signal` of kind `call`); recording starts only when `signal.result.offline` is empty for the devices in the
   call. If not: "Not recording. Mesail could not tell everyone in the call: 1 device did not get the notice.
   Nothing was recorded." with **Try again**.
- **The banner** (`.ms-rec-banner`, `--ms-recording-bg`): full width under the call header, for everyone, with **no
  close button** while the recording runs. Others: "Recording by Ana Ruiz" and "The recording is saved only on Ana
  Ruiz's device. Mesail does not keep it." The recorder: "You are recording this call", "Saved only on this computer ·
  08:12 · 96 MB written" and **Stop recording**. Phones: the same, shorter. Two recorders: one banner, "Recording by
  Ana Ruiz and Chen Wu". It is a `role="status"` line: heard once when it appears ("Ana Ruiz is recording this
  call."), then it stays first in the reading order.
- **People who join later** see it before they join (on the ringing card and on the call row next to Join) and in
  the call from the first second: the recorder's device sends the current state to each device that joins.
- **Stop:** the recorder gets "Recording saved: design-call-2026-11-14-1004.webm (412 MB)" with **Show in Finder**
  (Show in folder on Windows, Open on phones); everyone else sees the banner go and "Ana Ruiz stopped recording."
- **Leaving while recording** asks "Leave and stop recording?" ("Mesail saves what was recorded in …") with
  **Stay in the call** / **Leave and stop**. A recorder whose device drops out ends the recording; the banner goes
  for everyone with "Ana Ruiz's recording ended: Ana left the call." Back in the call, recording does not resume by
  itself: they start again and everyone is told again.
- **Write failure** (disk full, P7 `failed`): "Recording stopped. Mesail could not write the file; the disk may be
  full. The file holds what was recorded until 10:42." (`role="alert"`).

### 18.6 Call history in the chat (AC-CL-4, AC-CL-5)

- Three system rows, written by the caller's device as encrypted chat messages; each device words them for its
  own person. An icon tile, a title, a detail line and one button:
  - **Started** (green tile, while the call runs): "Ana Ruiz started a call", "Video call · 09:30 · You and Ana are
    in it", **Join**.
  - **Ended:** "Call ended · 12 min", "09:30 to 09:42" (groups add "9 people joined").
  - **Missed** (red tile, `phone-missed`): the person rung sees "Missed call from Ana Ruiz" with **Call back**; the
    caller sees "Bo Lee did not answer" with **Call again**. A group call nobody joined: "Missed call from Chen Wu",
    "Nobody joined". A declined call shows as missed.
- Duration words: "less than 1 min", "12 min", "1 h 05 min" (screen readers: "12 minutes", "1 hour 5 minutes").
- Rows are not messages: no reactions, threads or edits. In lists under 22rem the button moves under the words.
- **Sidebar:** a missed call makes the conversation unread ("Missed call" as its second line, a count of 1); a
  running call shows a green phone mark ("Call in progress"). **Activity** lists "Missed call from Bo Lee" with
  **Call back**.

### 18.7 Permissions, not supported, connection problems (AC-CL-3, AC-CL-9, AC-UI-4)

- **Before the browser asks** (first call in this browser): "Mesail needs your microphone and camera for this call.
  Your browser now asks. Choose Allow." Mesail asks only when a call starts or is accepted, never on page load.
- **Blocked microphone:** the call goes on (listening); the Mute button keeps a warning mark and its description
  points at a box "Mesail cannot use your microphone. It is blocked in this browser's settings. Others cannot hear
  you. To allow it: select the icon left of the address, then Microphone, then Allow." with **Check again**.
- **Blocked camera:** the call goes on with sound; "… so the others see your picture instead. You can still talk."
- **In the apps** the words name the system's own setting (macOS System Settings > Privacy & Security >
  Microphone / Camera / Screen & System Audio Recording, then quit and open Mesail again for the screen; Windows
  Settings > Privacy & security > Microphone; iPhone and iPad Settings > Mesail; Android Settings > Apps > Mesail >
  Permissions), with **Open settings** where the system can open that page.
- **Screen sharing:** nothing chosen in the picker: "Not sharing. Nothing was chosen." (info); refused by the system:
  "Mesail cannot share your screen." with where to allow it.
- **No device / in use:** "No microphone found. Plug one in, or stay in the call to listen."; "Your camera is in use
  by another app. Close that app, then choose Camera off again to turn it on."
- **Not supported (AC-CL-9):** a link or button into a call opens "Calls are not available on this device" with where
  calls work and **Back to #design**.
- **Connection:** "Joining the call…"; "Reconnecting… Your network dropped. You are still in the call." (tiles freeze
  and dim; 30 seconds); "Your connection is weak, so others may not see your video." with **Turn camera off**;
  "Could not connect to the call. Your network may block calls. Try again. If it keeps failing, ask your admin to
  check that the network allows Mesail calls." with **Try again** / **Leave**; after reconnecting fails: "You left
  the call because the connection was lost. The call in #design goes on." with **Rejoin**.
- **Full or ended:** `409 call_full`: "This call is full. 50 people are in it, the most a call can hold. Try again when
  someone leaves."; "This call has ended."
- **Removed from the conversation (AC-CL-3):** out at once: "You left the call in #design. You are no longer a member
  of #design, so you cannot be in its calls. Later calls in #design do not ring for you."
- **Switching domain during a call:** "Leave the call and switch to example.org?" ("Calls stay inside one domain.");
  the sign-out confirmation adds "Signing out also ends your call."
- **Phones in the background:** sound goes on where TACHYON P5 allows; the camera pauses and others see "Camera
  paused"; where the phone stopped the sound too: "Your phone paused the call while Mesail was in the background."

### 18.8 Rules for every call screen

- **Accessibility:** the call view is a `main` landmark with an `h1`; tiles are `role="img"` with names that carry
  their state; the bar is a `role="toolbar"`; F6 moves between regions; a skip link jumps to the controls. New
  people joining and leaving are announced politely and batched ("Bo Lee joined", at most one line every 3
  seconds). No state is colour alone: speaker ring plus icon, muted slash, recording dot plus words, inverted plus
  slashed control faces.
- **Contrast:** every pair on the stage is checked in TOKENS.md "Calls" (stage text 17:1, controls' borders 3.6:1,
  Leave 5.5:1, recording banner 6.5:1; high contrast: black stage, white borders, yellow speaker ring).
- **Motion:** the ringing pulse and the recording dot stop with `prefers-reduced-motion`.
- **Sizes:** controls 48 px faces (44 px floor on touch and under 768 px); answer buttons 56 px; every new target at
  least 44 px on touch. Checked in headless Chromium at 320, 375, 768, 1024, 1440 and 2560 px, light, dark and high
  contrast, coarse pointer at 375, 768 and 1024: no sideways scroll, no small target, every icon known.
- **New icons** in `mockups/build/mesail-icons.js` (the same 2 px stroke on the 24 px grid): `phone`, `phone-off`,
  `phone-incoming`, `phone-missed`, `video`, `video-off`, `mic`, `mic-off`, `screen-share`, `screen-share-stop`,
  `record`, `stop`, `volume`, `audio-lines`, `signal-low`, `layout-grid`, `picture-in-picture`.
