# Mesail UX specification

- Status: Draft for implementation
- Date: 2026-11-10
- Owner: Senior UX/UI Designer (Mesail rewrite)
- Product spec: [../SPEC.md](../SPEC.md) (scenarios S1-S12, acceptance criteria AC-*)
- Design tokens: [TOKENS.md](TOKENS.md)
- Component inventory and Tac mapping: [COMPONENTS.md](COMPONENTS.md)
- Mockups: [mockups/index.html](mockups/index.html) (open the file in a browser; add
  `?theme=dark`, `?theme=high-contrast` or, on a narrow window, `?pane=list|main|thread`)

This document says how Mesail looks and behaves. The Go Tac frontend slices build against it.
Where this document and SPEC.md disagree, SPEC.md decides behaviour and this document decides
presentation; open questions are listed in section 13.

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

| Width | Layout |
| --- | --- |
| 1200 px and wider | Rail (68 px), sidebar (272 px), conversation (flexible), thread or details pane (400 px) |
| 768-1199 px | Rail, sidebar, conversation. The thread pane covers the conversation (with a shadow) and closes with Esc or the close button. |
| under 768 px | One pane at a time: list, conversation or thread. A back button in each header. Bottom navigation on the list: Home, Activity, Search, You. Dialogs become bottom sheets. |

- The sidebar sections, in order: Activity, Threads, **Channels**, **Direct messages**, **Email**.
  The Email section holds direct conversations that include anyone outside the domain. Channels
  with an email address show a small mail icon.
- Sidebar row states: default, hover, focus, current (`aria-current="page"`), unread (bold),
  mention count (red badge), muted (dimmed, bell-off icon).
- Safe areas: headers pad `env(safe-area-inset-top)`; the composer, sidebar footer, bottom
  navigation and sheets pad `env(safe-area-inset-bottom)`; side insets apply in landscape.

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

## 8. Sign-in (Keycloak, OpenID Connect)

Mesail shows no password field of its own (AC-ID-5).
1. `signin.html`: the user types a work email (or picks a recent domain).
2. Mesail finds the domain and says where the user goes next ("You will go to
   sign-in.example.com"). Unknown or disabled domains get a clear message.
3. The browser goes to Keycloak with `login_hint`; Keycloak handles passwords, passkeys and
   second factors.
4. Back in Mesail: "Finishing sign-in…", then the app.
Errors (cancelled, account turned off, Keycloak unreachable, expired sign-in attempt), sign-out
per domain, and the "session ended" dialog (the draft is kept) are all designed.
Mockups: `signin*.html`, `signed-out.html`, `session-expired.html`.

## 9. Settings and admin

- **Settings** (per domain): profile (name comes from Keycloak, so it links to the Keycloak account
  page), notifications (push permission states, email summary delay, do-not-disturb schedule),
  email (send-from aliases, signature, remote images, Cc default), appearance (DuVay themes light,
  dark, auto, high contrast; density; text size; motion).
- **Admin** (role `mesail-admin`, checked by the server; the UI only hides entry points):
  domains with DNS setup and checks, DKIM rotation, members (managed in Keycloak), channel email
  addresses, mail queue and suppression list. A user without the role sees a plain "You need admin
  rights" page.
- Mockups: `settings*.html`, `admin-*.html`.

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
- Targets: 24 px minimum with a pointer, 44 px on touch screens and under 768 px.
- Motion: 120-200 ms; everything respects `prefers-reduced-motion`.
- Text never below 12 px; body 15 px at 1.5 line height.

## 11. DuVay: use it, selectively

**Decision: Mesail uses DuVay.** CADUCEUS already loads DuVay, and DuVay's default palette came from
CADUCEUS, so the brand carries over.

- **Tokens: yes, all of them.** Mesail's colours, type, spacing, radii, focus ring, motion and
  breakpoints are DuVay `--w-*` tokens, and themes switch with DuVay's `w-theme` attribute
  (light, dark, auto, high-contrast). Mesail adds a small `--ms-*` layer only for what DuVay lacks:
  delivery colours (chat, email), presence colours, the domain rail and layout sizes (TOKENS.md).
- **CSS: core plus the component files Mesail uses**, not the full `duvay.min.css`
  (about 554 KiB). Import `core.css`, `tokens.css`, `themes.css` and the component files listed in
  COMPONENTS.md (buttons, inputs, chips, badges, avatars, menus, dialogs, snackbars, skeletons,
  tooltips, tabs, switches, empty states, command palette).
- **Web components: no.** The `<w-*>` bundle is about 754 KiB of JavaScript, which would break the
  per-route script budget (AC-UI-3) and duplicates what Tac components do. Tac components render
  DuVay's class names (`w-btn`, `w-chip`, ...) directly.
- **Mesail-specific components** (recipient chip, composer, message, email card, domain rail,
  delivery line) are Tac components styled only with tokens. Generic parts of them (chip base,
  badge, avatar, menu) reuse DuVay classes.
- The mockups use their own small `ms-*` CSS on top of the real DuVay tokens, so they open without
  a build. COMPONENTS.md maps each `ms-*` class to the DuVay class and the Tac component.

## 12. Design decisions made while building the mockups

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
- **The sign-in field accepts a work email or a bare domain** (`type="text"`, `inputmode="email"`).
- **Mockup-only helpers** (labelled state frames, `?theme=`, `?pane=`) are not part of the product.

## 13. Open questions

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
- **Sign-in.** Are drafts kept on the device or on the server, and does sign-out delete them? Is
  "Remember this domain on this device" on by default? Who turns a domain off: the operator or the
  domain admin?
- **Settings.** Does the job title come from Keycloak, like the name? Is "Always show images from
  people I have emailed" allowed (AC-IN-5 speaks of one sender at a time)? Is auto-reply out of the
  first release (the mockup shows it as "Later release")?
- **Search.** Do People hits match names and addresses only? Are recent searches kept on the device?
- **Admin.** Does "change role" write to Keycloak through its admin API (mockup) or only link out?
  Is the suppression list per domain (expected, now that the domain is the tenant)? Is mail to an
  archived channel's address refused (mockup) or accepted? Who can archive and unarchive? What
  happens to a turned-off domain (mockup: mail refused, sign-in blocked, messages kept)? When can
  the old DKIM record go (mockup: after 7 days)?
