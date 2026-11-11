# Mesail component inventory

Each row maps a mockup component to the DuVay class it builds on and to the Tac Go file that will
implement it. Paths follow mesail-arch's skeleton and slice plan (`docs/mesail/SLICES.md`):

- web app: `apps/web/client/` (slices F1-F4); admin app: `apps/web-admin/client/` (slice F5);
- a component is `components/<group>/<name>/tac.html` (+ `tac.css`), used as the tag
  `<group-name>`; a page is `pages/<route>/tac.html`; state and behaviour live in a
  `services/<Name>Service.go` with Tachyon's `// @Service` marker.

Groups that already exist in the skeleton: `app`, `sidebar`, `conversation`, `composer`, `thread`,
`search`, `notifications` (web) and `admin` (admin app). Rows marked **new group** need a slice owner;
see "Gaps" at the end.

Building rule for every Tac component (README section 11): DuVay v26.35.01-1 `<w-*>` web components for
every generic control, Tailwind CSS 4.3.5 utilities in the component's `tac.css` for layout and spacing,
and Mesail classes (`mockups/assets/mesail-components.css`) only where no DuVay element fits. The
"DuVay element(s)" column says what each component is built from; "none fits" marks a Mesail component.
The mockup is the reference for look and states.

## Shared building blocks

| Component | Mesail class | DuVay element(s) | Tac target | Slice |
| --- | --- | --- | --- | --- |
| Button (default, primary, email, danger, ghost, sizes; hover, active, disabled, busy) | none | `w-btn` (variant filled, tonal, outlined, text; `color="danger"`; `loading`) | the DuVay element in markup; no Mesail component | all |
| Icon button | none | `w-icon-btn` | the DuVay element | all |
| Icon | `.ms-icon-sm`, `.ms-icon-xs` (sizes) | `w-icon` with Mesail's `ms` SVG set | `apps/web/client/shared/mesail-icons.js` (registers the `ms` set with DuVay) | F1 |
| Text field, select, textarea, field with help and error | none | `w-text-field`, `w-select`, `w-textarea`, `w-messages` | the DuVay elements | all |
| Checkbox, switch | none | `w-checkbox`, `w-switch` | the DuVay elements | all |
| Badge (count, mention), unread dot | `ms-count` | `w-badge` (`ms-count` for neutral) | the DuVay element | F2 |
| Tag (Chat, Email, OK, warning, danger) | `.ms-how--chat`, `--email`, `--both`, `--checking`, `--invalid`, `--warn` | `w-chip size="small"` + `.ms-how--*` | `components/app/tag/tac.html` `<app-tag>` | F1 |
| Avatar (sizes, external, channel) | none | `w-avatar`, `w-avatar-group` | `components/app/avatar/tac.html` `<app-avatar>` | F1 |
| Presence mark (active, idle, dnd, offline) | `ms-offline`, `ms-external` on `w-avatar` | `w-avatar status` + Mesail shapes (no DuVay idle/dnd/offline) | `components/app/presence/tac.html` `<app-presence>` | F4 |
| Menu and popover (radio items, disabled with reason, danger) | none | `w-menu` (menuitemradio), `w-popover` | `components/app/menu/tac.html` `<app-menu>` | F1 |
| Tooltip | none | `w-tooltip` | the DuVay element | all |
| Dialog and bottom sheet | none | `w-dialog`, `w-alert-dialog`, `w-bottom-sheet` | `components/app/dialog/tac.html` `<app-dialog>` | F1 |
| Toast | none | `w-snackbar`, `w-snackbar-queue` | `components/app/toasts/tac.html` `<app-toasts>` + `ToastService.go` | F1 |
| Banner (info, email, warning, error) | none | `w-banner`, `w-alert` | `components/app/banner/tac.html` `<app-banner>` | F1 |
| Skeleton, spinner, progress | none | `w-skeleton-loader`, `w-progress-circular`, `w-progress-linear` | the DuVay elements | all |
| Empty and error state | none | `w-empty-state` | `components/app/empty/tac.html` `<app-empty>` | F1 |
| Tabs | none | `w-tabs` + `w-tab` (add tabpanel roles) | the DuVay elements | F4, settings |
| Card, row list, table | none | `w-card`, `w-list`, `w-table` | the DuVay elements | settings, F5 |
| Keyboard key | none | `w-kbd`, `w-hotkey` | the DuVay element | all |

## App shell and navigation (F1, F2)

| Component | Mesail class | Mockups | Tac target | Slice | DuVay element(s) |
| --- | --- | --- | --- | --- | --- |
| App shell (rail, sidebar, main, thread, bottom nav; one pane under 600 px) | none (DuVay and Tailwind) | `messenger.html` | `components/app/shell/tac.html` `<app-shell>` + `ShellService.go` (current pane, current domain) | F1 | Tailwind grid; `w-bottom-navigation` |
| Skip links and live announcer | `.ms-skip-link` | `messenger.html` | inside `<app-shell>`; `AnnounceService.go` | F1 | none (plain links, Tailwind `sr-only`) |
| Domain rail and domain tile | `.ms-rail`, `.ms-domain-tile` | `messenger.html`, `domain-switcher.html` | `components/app/rail/tac.html` `<app-rail>` + `DomainService.go` | F1 | none fits: Mesail tile + `w-badge`, `w-icon` |
| Domain menu (switch from `Session.domains`, default first, Default and Admin tags, default-domain link, one Sign out) and its phone sheet | none (DuVay and Tailwind) | `domain-switcher.html`, `domain-switcher-mobile.html`, `domain-switch-states.html` | `components/app/domain-menu/tac.html` `<app-domain-menu>` | F1 | `w-menu` + `w-list`; `w-bottom-sheet` on phones |
| Sidebar (head, search button, sections, footer) | `.ms-sidebar*` | `messenger.html` | `components/sidebar/nav/tac.html` `<sidebar-nav>` + `SidebarService.go` | F2 | `w-list nav`, `w-btn`, `w-icon-btn`, `w-hotkey` |
| Sidebar section (collapsible, add button) | none (DuVay and Tailwind) | `messenger.html` | `components/sidebar/section/tac.html` `<sidebar-section>` | F2 | Mesail toggle + `w-icon-btn` + `w-list` |
| Sidebar row (channel, DM, group DM, email conversation; current, unread, mention, muted) | none (DuVay and Tailwind) | `messenger.html` | `components/sidebar/item/tac.html` `<sidebar-item>` | F2 | `w-list-item href` + `w-avatar`/`w-icon` + `w-badge` |
| Me button (avatar, presence, address) | none (DuVay and Tailwind) | `messenger.html`, `presence-menu.html` | `components/sidebar/me/tac.html` `<sidebar-me>` | F2 | `w-avatar status` + `w-menu` |
| Bottom navigation (phones) | none (DuVay and Tailwind) | `messenger.html`, `mobile-home.html` | `components/app/bottom-nav/tac.html` `<app-bottom-nav>` | F1 | `w-bottom-navigation grow` + `w-bottom-nav-item` + `w-icon` |

## Conversation (F2)

| Component | Mesail class | Mockups | Tac target | DuVay element(s) |
| --- | --- | --- | --- | --- |
| Conversation view (header + timeline + composer slot) | none (DuVay and Tailwind) | `messenger.html` | `components/conversation/view/tac.html` `<conversation-view>` + `ConversationService.go` | Tailwind flex column with `@container` |
| Conversation header (title, topic, members, actions, back) | none (DuVay and Tailwind) | `messenger.html` | `components/conversation/header/tac.html` `<conversation-header>` | `w-icon-btn`, `w-btn` |
| Timeline (`role="log"`, day divider, New divider, jump to new) | `.ms-day`, `.ms-new-divider` | `messenger.html`, `states-error.html` | `components/conversation/timeline/tac.html` `<conversation-timeline>` + `RealtimeService.go` | none fits: Mesail log + `w-infinite-scroll side="start"` |
| Message (grouped, mention, highlight, system, pending, failed, deleted, edited) | `.ms-msg*` | `messenger.html`, `delivery-status.html`, `message-delete.html` | `components/conversation/message/tac.html` `<conversation-message>` | none fits: `.ms-msg` + `w-avatar`, `w-chip`, `w-alert` |
| Message actions toolbar and phone action sheet | `.ms-msg__actions` | `messenger.html`, `mobile-conversation.html` | `components/conversation/actions/tac.html` `<conversation-actions>` | `w-icon-btn` toolbar; `w-bottom-sheet` + `w-list` on touch |
| Reactions | none (DuVay and Tailwind) | `messenger.html` | `components/conversation/reactions/tac.html` `<conversation-reactions>` | `w-chip size="small" selected` + `w-icon-btn` |
| Thread summary link | `.ms-thread-link` | `messenger.html` | part of `<conversation-message>` | Mesail link + `w-avatar` |
| Attachment (file card) | `.ms-file*` | `messenger.html`, `email-conversation.html` | `components/conversation/file/tac.html` `<conversation-file>` | Mesail `.ms-file` (DuVay `.w-attachment` is CSS only) |
| Delivery line and delivery details | `.ms-delivery*` | `delivery-status.html` | `components/conversation/delivery/tac.html` `<conversation-delivery>` | Mesail line + `w-icon`; details in `w-popover` |
| Email card (subject, From/To/Cc, sender check, quoted text, remote images) | `.ms-email-card*` | `email-conversation.html`, `email-inbound-new.html` | `components/conversation/email-card/tac.html` `<conversation-email-card>` | `.ms-email-card` + `w-chip`, `w-expansion-panels` |
| Banners in a conversation (outside people, archived, offline) | none (DuVay and Tailwind) | `email-conversation.html`, `channel-details.html`, `states-error.html` | `<app-banner>` | `w-banner` |

## Composer (F3)

| Component | Mesail class | Mockups | Tac target | DuVay element(s) |
| --- | --- | --- | --- | --- |
| Composer (chat mode, email mode, reply mode) | `.ms-composer*` | `messenger.html`, `compose-*.html` | `components/composer/box/tac.html` `<composer-box>` + `ComposerService.go` | `.ms-composer` frame + `w-icon-btn`, `w-btn` |
| Recipient row (To, Cc, Bcc) with input and suggestions | `.ms-composer__row`, `.ms-recipients`, `.ms-suggest*` | `compose-new.html` | `components/composer/recipients/tac.html` `<composer-recipients>` + `RecipientService.go` (calls `/api/v1/recipients/resolve`) | `w-combobox` (multiple, chips, no-filter) or Mesail row + `w-chip` |
| Recipient chip (chat, email, both, resolving, invalid, warning) | none (DuVay and Tailwind) | `compose-*.html` | `components/composer/chip/tac.html` `<composer-chip>` | none fits: Mesail chip (a `<button aria-haspopup="menu">` with DuVay chip classes, a separate remove button) + `.ms-how--*` (README section 11) |
| Chip delivery menu (Chat / Email / Chat and email, disabled reason) | none (DuVay and Tailwind) | `compose-chip-menu.html` | `components/composer/chip-menu/tac.html` `<composer-chip-menu>` | `w-menu` (menuitemradio) |
| Email fields (From, Subject, Cc/Bcc toggles) and email mode bar | `.ms-composer__mode--email` | `compose-email.html`, `compose-mixed.html` | `components/composer/email-fields/tac.html` `<composer-email-fields>` | `w-text-field`, `w-select`, `w-btn` |
| Send summary (who gets what; error variant) | `.ms-composer__summary*` | `compose-mixed.html`, `compose-errors.html` | `components/composer/summary/tac.html` `<composer-summary>` | `w-alert` / Mesail summary + `w-icon` |
| Attachment chips with progress | `.ms-attach-list`, `.ms-attach-chip` | `compose-email.html` | `components/composer/attachments/tac.html` `<composer-attachments>` + `UploadService.go` | `w-chip` + `w-progress-linear`; `w-file-upload` for drop |
| Signature block | `.ms-signature` | `compose-email.html` | part of `<composer-email-fields>` | `w-textarea` (settings), static text in composer |
| Send button (chat, email, mixed label, disabled) | `w-btn.ms-btn-email` | `compose-*.html` | part of `<composer-box>` | `w-btn` filled (email colour via Mesail class) |
| Recipients-changed dialog (409) | none (DuVay and Tailwind) | `compose-errors.html` | `components/composer/changed/tac.html` `<composer-changed>` | `w-dialog` |

## Threads, search, activity, presence (F4)

| Component | Mockups | Tac target | DuVay element(s) |
| --- | --- | --- | --- |
| Thread pane (root, replies divider, composer with "Also send to #channel") | `messenger.html` | `components/thread/pane/tac.html` `<thread-pane>` + `ThreadService.go` | Tailwind pane + `w-checkbox` |
| Search palette (combobox + listbox, recent searches) | `search-palette.html` | `components/search/palette/tac.html` `<search-palette>` + `SearchService.go` | `w-command-palette` |
| Search page, filter bar, result row | `search.html`, `search-states.html` | `components/search/panel/tac.html` `<search-panel>`, `components/search/filter-bar/tac.html`, `components/search/result/tac.html` | `w-text-field`, `w-chip-group`, `w-list` |
| Activity list, tabs, notification row | `activity.html` | `components/notifications/list/tac.html` `<notifications-list>`, `components/notifications/item/tac.html` + `NotificationService.go` | `w-tabs`, `w-list`, `w-list-item` |
| Presence and account menu (do not disturb, status message, sign out) | `presence-menu.html` | `components/notifications/presence-menu/tac.html` + `PresenceService.go` | `w-menu`, `w-list`, `w-switch` |
| Profile card (same domain: Message + Email; other domain or outside: email only) | `profile-card.html`, `cross-domain.html` | `components/app/profile-card/tac.html` `<app-profile-card>` (F1 owns it, F2-F4 open it) | `w-hover-card` + `w-avatar`, `w-btn` |

## Email-specific parts (F2 and F3)

| Component | Mockups | Tac target | DuVay element(s) |
| --- | --- | --- | --- |
| Participants bar (who is in, and how: Chat or Email) | `email-conversation.html` | `components/conversation/participants/tac.html` | `w-avatar-group`, `w-chip` |
| Sender check (SPF, DKIM, DMARC; verified or warning, with details) | `email-conversation.html`, `email-inbound-new.html` | `components/conversation/email-auth/tac.html` | `w-chip` + `w-popover` |
| Hidden images and unverified-sender notice | `email-inbound-new.html` | `components/conversation/email-notice/tac.html` | `w-alert` |
| "Only people in example.com see this" marker | `email-conversation.html` | part of `<conversation-message>` | Mesail marker + `w-icon` |
| Show original dialog (headers, plain text, download .eml) | `email-original.html` | `components/conversation/original/tac.html` + `EmailService.go` | `w-dialog fullscreen scrollable`, `w-tabs` |
| Reply mode bar (Reply all / Reply to one; collapsed subject) | `compose-reply-email.html`, `email-conversation.html` | `components/composer/reply-mode/tac.html` | `w-btn-toggle` |
| "Send to customer" choice in a channel thread from email | `channel-email.html` | `components/composer/send-to/tac.html` | `w-checkbox` |
| Domain explainer and "How domains work" dialog | `cross-domain.html`, `compose-mixed.html` | `components/app/domain-explainer/tac.html` (F1) | `w-dialog`, `w-alert` |

## Delivery and errors (F2, F3)

| Component | Mockups | Tac target | DuVay element(s) |
| --- | --- | --- | --- |
| Delivery details popover (each recipient, Chat or Email, status, times) | `delivery-status.html` | `components/conversation/delivery-details/tac.html` + `DeliveryService.go` | `w-popover` + `w-table` |
| Bounce or failure notice on a message, with actions | `delivery-status.html` | `components/conversation/notice/tac.html` | `w-alert` + `w-btn` |
| Failed chat send row (Retry, Delete; same `clientMessageId`) | `delivery-status.html`, `states-error.html` | `components/conversation/retry/tac.html` | `w-alert` + `w-btn` |
| Composer note (error, warning, info inside the composer) | `compose-errors.html` | `components/composer/note/tac.html` | `w-alert` (density compact) |
| Attachment size meter | `compose-errors.html` | `components/composer/attach-meter/tac.html` | `w-progress-linear` |
| Delete confirm with the "email cannot be recalled" warning | `message-delete.html` | `<app-dialog>` used by `<conversation-actions>` | `w-alert-dialog` |
| Connection banner (offline, reconnecting) and jump-to-new pill | `states-error.html`, `session-expired.html` | `components/app/connection/tac.html` + `ConnectionService.go`; `components/conversation/jump-pill/tac.html` | `w-banner sticky`; `w-btn` pill |

## Channels (F2; channel admin in F5)

| Component | Mockups | Tac target | DuVay element(s) |
| --- | --- | --- | --- |
| Channel browser and row (Join, Joined, Joining) | `channel-browse.html` | `components/sidebar/channel-browser/tac.html`, `components/sidebar/channel-row/tac.html` | `w-text-field`, `w-list`, `w-btn` |
| Create channel dialog (name rules, Public or Private, add people) | `channel-create.html` | `components/sidebar/channel-create/tac.html` + `ChannelService.go` | `w-dialog`, `w-text-field`, `w-radio-group`, `w-combobox` |
| Channel details panel (about, email address, members, notifications, leave) | `channel-details.html` | `components/thread/channel-details/tac.html` (uses the thread pane slot) | Tailwind pane + `w-list`, `w-switch`, `w-btn-toggle` |
| Archived notice in place of the composer | `channel-details.html` | `components/conversation/readonly-notice/tac.html` | `w-alert` |

## Sign-in and session (F1)

| Component | Mockups | Tac target | DuVay element(s) |
| --- | --- | --- | --- |
| Sign-in form (L6: one field "Username or email address", password, optional 2-step code, remember username, 401 / 403 not_a_member / 403 no_domain / 429 / 503) | `signin.html`, `signed-out.html` | `pages/signin/tac.html`, `pages/signed-out/tac.html`, `components/app/signin-form/tac.html` + `SessionService.go` (POST to the identity service) | `w-text-field` (code too: `w-otp-input` breaks password-manager fill), `w-checkbox`, `w-btn`, `w-alert` |
| Forced password change (L6, `mustChangePassword`) | `password-change-required.html` | `pages/password-required/tac.html` + `SessionService.go` | `w-text-field`, `w-btn`, `w-alert` |
| No-domain and access-ended screens (L6) | `domain-switch-states.html` | `components/app/domain-gone/tac.html` | `w-btn`, Mesail notice box |
| Default domain setting (L6, `PUT /directory/me/default-domain`) | `settings.html` (`#default-domain`) | `components/settings/default-domain/tac.html` | `w-select` |
| Wordmark | sign-in pages | `components/app/wordmark/tac.html` | none (Mesail mark) |
| Session ended dialog (draft kept) | `session-expired.html` | `components/app/session-ended/tac.html` | `w-dialog persistent` |

## Settings (new group: no slice owns it yet)

| Component | Mockups | Tac target | DuVay element(s) |
| --- | --- | --- | --- |
| Settings page frame (nav, top tabs under 1100 px, save status) | `settings*.html` | `pages/settings/tac.html` + `SettingsService.go` | `w-list nav` (md and up), `w-tabs` (narrow) |
| Save status (one live region: Saving, Saved, Not saved) | `settings*.html` | `components/settings/save-status/tac.html` | `w-messages` / live region |
| Settings row, radio cards, segmented choice | `settings*.html`, `channel-create.html` | `components/settings/row/tac.html`, `components/app/choice/tac.html`, `components/app/segmented/tac.html` | `w-radio-group`, `w-btn-toggle`, `w-card` |
| Push permission panel (not asked, on, blocked, not supported) | `settings-notifications.html` | `components/settings/push-permission/tac.html` + `PushService.go` | `w-alert`, `w-btn` |
| Quiet hours schedule | `settings-notifications.html` | `components/settings/schedule/tac.html` | `w-chip-group` (days), `w-select` (hours), `w-switch` |
| Signature editor, send-from addresses | `settings-email.html` | `components/settings/signature/tac.html`, `components/settings/send-from/tac.html` | `w-textarea`, `w-radio-group`, `w-select` |
| Theme picker (DuVay `w-theme`), density, text size, motion | `settings-appearance.html` | `components/settings/theme-picker/tac.html` | `w-radio-group` cards, `w-btn-toggle`, `w-switch` |
| Keyboard shortcuts list | `settings-appearance.html` | `components/settings/shortcuts/tac.html` | `w-list` + `w-hotkey` |
| Account security rows and dialogs (change password; turn on 2-step with QR and key; set up on a new phone; turn off) | none (DuVay and Tailwind) | `settings.html`, `settings-security.html` | `components/settings/account-security/tac.html`, `components/settings/totp-setup/tac.html` + `AccountSecurityService.go` (identity `password` and `totp` routes) | `w-dialog` (full screen under 600 px), `w-text-field`, `w-btn`, `w-alert`, `.ms-tag`, `w-snackbar`, `w-skeleton-loader` |

## Admin app (F5: `apps/web-admin/client/`)

| Component | Mockups | Tac target | DuVay element(s) |
| --- | --- | --- | --- |
| Admin shell and nav | `admin-*.html` | `components/admin/shell/tac.html` (exists), `components/admin/nav/tac.html` | Tailwind grid + `w-list nav` |
| Domains list (table on desktop, cards on phones) | `admin-domains.html` | `pages/domains/tac.html`, `components/admin/domain-list/tac.html` + `AdminDomainService.go` | `w-table` (cards via container query) + `w-chip` |
| Add domain dialog | `admin-domain-add.html` | `components/admin/domain-add/tac.html` | `w-dialog`, `w-text-field` |
| DNS records, status marks, Check again, DKIM rotation, danger zone | `admin-domain-dns.html` | `pages/domains/dns/tac.html`, `components/admin/dns-record/tac.html`, `components/admin/dns-check/tac.html`, `components/admin/dkim-rotation/tac.html`, `components/admin/danger-zone/tac.html` | `w-text-field readonly` + `w-icon-btn`, `w-chip`, `w-btn`, `w-card` |
| Typed confirmation (type the domain name) | `admin-domain-dns.html` | `components/admin/confirm-typed/tac.html` | `w-alert-dialog` + `w-text-field` |
| Copy field | `admin-domain-dns.html`, `channel-details.html` | `components/admin/copy-field/tac.html` (web app copy in `components/app/copy-field/`) | `w-text-field readonly` + `w-icon-btn` |
| Admin sign-in (L7: the same one-field form, "Admin" tag) and "You need admin rights" | `admin-signin.html` | `components/admin/gate/tac.html` (exists) | `w-text-field`, `w-btn` |
| Members table (L7: username, address, Admin tag, row menu: role, account actions, remove) | `admin-members.html`, `admin-members-states.html` | `pages/members/tac.html`, `components/admin/member-row/tac.html` + `AdminMemberService.go` | semantic `<table>` (`.ms-rtable`, cards under 48rem of page), `w-menu` |
| Add member dialog (L7: username first; 400 `displayName` opens step 2 for a new person; existing-person notice) | `admin-member-add.html` | `components/admin/member-add/tac.html` | `w-dialog`, `w-text-field` (`suffix`), `w-checkbox` |
| Member actions (L7: remove, `409 last_admin`, reset password, remove 2-step code, turn off, rename, `403` AC-ID-13) | `admin-member-actions.html` | `components/admin/member-actions/tac.html` | `w-dialog`, `w-alert-dialog` |
| People page for Operators (L7: find, domains with admin marks, state, actions, delete with typed username) | `admin-people.html` | `pages/people/tac.html`, `components/admin/person-row/tac.html` + `AdminPeopleService.go` | `.ms-rtable`, `w-select`, `w-menu`, `w-dialog` |
| First admin of a domain (L7, Operator, `POST /directory/domains/{domainId}/members`) | `admin-domain-first-admin.html` | `components/admin/first-admin/tac.html` | `w-text-field`, `w-btn`, `w-dialog` |
| No access page (403) | `admin-members-states.html` | `pages/no-access/tac.html` | `w-empty-state` |
| Channel addresses and archive | `admin-channels.html` | `pages/channels/tac.html`, `components/admin/channel-address/tac.html` + `AdminChannelService.go` | `w-table`, `w-dialog`, `w-text-field` |
| Mail delivery: queue numbers, suppression list, limits | `admin-mail.html` | `pages/mail/tac.html`, `components/admin/queue-stats/tac.html`, `components/admin/suppression-list/tac.html`, `components/admin/limits-form/tac.html` + `MailOpsService.go` | `w-card` stats, `w-table`, `w-text-field` |

## Gaps for mesail-arch

- **Settings** (`components/settings/**`, `pages/settings/**`, `SettingsService.go`, `PushService.go`)
  is in no slice. Suggest adding it to F4 (notifications) or a new slice F6.
- **Phone layout pieces** (`<app-bottom-nav>`, the "New message" button, the action sheet) belong
  to F1 and F2; SLICES.md does not list them yet.
- **Screen CSS to merge.** Each mockup area has its own file in `mockups/assets/screens/`. When the
  Tac components are built, move each class into its component's `tac.css`. Three names are used
  twice and must become one: `.ms-profile-card` (`email.css`, `search.css`), the labelled-state frames
  (`.ms-states`, `.mk-state`, `.ms-mock-state`, `.ms-state`: mockup-only, do not port) and the
  invalid composer row (`.ms-attach-chip--error`, `.ms-composer__row--invalid` in `composer.css`
  and `delivery.css`).
- **Tables on phones.** `admin.css` turns table rows into cards with a container query. Tac must keep
  table semantics (`role="table"`, `row`, `cell`) when it does this, because `display: block`
  removes them in Safari.
