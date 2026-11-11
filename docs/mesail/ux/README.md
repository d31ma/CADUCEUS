# Mesail UX specification

- Status: Draft for implementation
- Date: 2026-11-10
- Owner: Senior UX/UI Designer (Mesail rewrite)
- Product spec: [../SPEC.md](../SPEC.md) (scenarios S1-S12, acceptance criteria AC-*)
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
`example.org` are two separate spaces, even inside one organization.

- **Rail (left edge, desktop and tablet):** one tile per domain the user is signed in to.
  The tile shows two letters, a mention count or an unread dot. The current domain has a bar on
  its left and an inverted tile. "+" signs in to another domain.
- **Sidebar header:** the domain name with a menu: switch domain, sign in to another domain,
  sign out of this domain, sign out of all domains. On narrow screens the rail is hidden and this
  menu opens as a bottom sheet.
- **Identity per domain:** Ana is `ana@example.com` in one space and `ana@example.org` in the
  other. Settings, presence and notifications are per domain.
- **Who can be chatted with:** only members of the current domain. Anyone else (another hosted
  domain, or outside) gets email. The recipient chip, the profile card and the composer summary
  all say why: "Chat stays inside example.com. finn@example.org gets an email."
- Mockups: `messenger.html`, `domain-switcher.html`, `cross-domain.html`.

## 3. Layout

Breakpoints are DuVay's (`--w-bp-*`), set as the Tailwind breakpoints: `sm` 600 px, `md` 960 px,
`lg` 1280 px, `xl` 1920 px, `2xl` 2560 px. Panes respond to their own width with container queries, so
the same component works in the conversation, the thread pane and a phone screen.

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

Changed on 2026-11-10 by the user's decision (relayed by slice F1): the browser never talks to
Keycloak. Mesail shows its own form and posts it to the identity service, which checks it with
Keycloak. Mesail still stores no password, TOTP secret or recovery code (AC-ID-5).

- **One card** (`signin.html`): Work email, Password (with a Show button), optional 2-step code
  ("Only if your account uses one."; TOTP in v1, no passkeys), "Remember my email on this device"
  (off by default, for shared computers), and **Sign in**. Recent domains below the form fill in
  the email and move focus to Password. "Forgot your password? Ask an admin of your domain to
  reset it."
- **One answer for every failure**, so the form never reveals whether an account exists:
  "The email, password or 2-step code is not right." The password field is cleared and focused.
- **Too many tries:** "Too many tries. Wait a minute, then try again." The button stays disabled
  until then. **Service down:** "We cannot reach sign-in right now. Try again in a minute."
- **Sign in to another domain** (from the domain menu): the same card with the title
  "Sign in to another domain", a line saying the user stays signed in to the current domain, and
  a Cancel button.
- **Sign-out** happens in place, per domain or for all domains (`signed-out.html`). The "session
  ended" dialog keeps the draft (`session-expired.html`).
- Connection banners use one wording everywhere: "Reconnecting… New messages may be late." with
  the button **Try now**.

## 9. Settings and admin

- **Settings** (per domain): profile (name and password are managed by the domain admin; Mesail
  shows them read-only), notifications (push permission states, email summary delay, do-not-disturb schedule),
  email (send-from aliases, signature, remote images, Cc default), appearance (DuVay themes light,
  dark, auto, high contrast; density; text size; motion).
- **Admin** (role `mesail-admin`, checked by the server; the UI only hides entry points):
  domains with DNS setup and checks, DKIM rotation, members (managed in Keycloak), channel email
  addresses, mail queue and suppression list. A user without the role sees a plain "You need admin
  rights" page.
- Mockups: `settings*.html`, `admin-*.html`.
- **Account security** (SPEC AC-ID-7, AC-ID-8; `settings.html`, `settings-security.html`): the
  "Account and sign-in" card has a **Password** row ("Change password") and a **2-step sign-in** row
  (status from `GET /api/v1/identity/totp`: On or Off; "Turn on", or "Set up on a new phone" and
  "Turn off"). Each opens a dialog (full screen under 600 px).
  - Change password: current password, new password (Show toggles, no repeat field), 2-step code
    when on. One message for a wrong password or code; the server's rule text for `password_policy`.
    Success keeps this session and says other devices were signed out.
  - Turn on: three steps (confirm password; scan the QR code or type the key, grouped by 4 with Copy;
    type the 6-digit code). "Set up on a new phone" also asks for the current code in step 1.
    No recovery codes: "If you lose your phone, ask an admin of example.com to reset 2-step sign-in."
  - Turn off: password and code, with the warning "Without it, anyone with your password can sign in."

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
  turns a domain off: the operator or the domain admin? Do admins still open the Keycloak console from
  `admin-members.html` ("Open Keycloak"), or does the admin app manage members through the identity
  service? (Resolved: people change their own password and 2-step code in Mesail's settings, AC-ID-7/8.)
- **Settings.** Does the job title come from Keycloak, like the name? Is "Always show images from
  people I have emailed" allowed (AC-IN-5 speaks of one sender at a time)? Is auto-reply out of the
  first release (the mockup shows it as "Later release")?
- **Search.** Do People hits match names and addresses only? Are recent searches kept on the device?
- **Admin.** Does "change role" write to Keycloak through its admin API (mockup) or only link out?
  Is the suppression list per domain (expected, now that the domain is the tenant)? Is mail to an
  archived channel's address refused (mockup) or accepted? Who can archive and unarchive? What
  happens to a turned-off domain (mockup: mail refused, sign-in blocked, messages kept)? When can
  the old DKIM record go (mockup: after 7 days)?
