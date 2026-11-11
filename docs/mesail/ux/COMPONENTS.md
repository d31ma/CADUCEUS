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

Styling rule for every Tac component: DuVay classes for generic parts (`w-btn`, `w-chip`,
`w-badge`, `w-avatar`, `w-menu`, `w-dialog`, ...), `--w-*` and `--ms-*` tokens for the rest, no raw
values. The mockup class is the reference for look and states.

## Shared building blocks

| Component | Mockup class | DuVay base | Tac target | Slice |
| --- | --- | --- | --- | --- |
| Button (default, primary, email, danger, ghost, sizes; hover, active, disabled, busy) | `.ms-btn*` | `w-btn` (`components/buttons.css`) | DuVay class in markup; no component | all |
| Icon button | `.ms-icon-btn` | `w-icon-btn` | DuVay class | all |
| Icon | `.ms-icon .ms-i-*` | DuVay icon set (`icons.js`) or an SVG sprite in `shared/` | `apps/web/client/shared/icons.svg` | F1 |
| Text field, select, textarea, field with help and error | `.ms-field`, `.ms-input`, `.ms-help`, `.ms-error-text` | `w-text-field`, `w-input`, `w-select` | DuVay classes | all |
| Checkbox, switch | `.ms-check`, `.ms-switch` | `w-checkbox`, `w-switch` | DuVay classes | all |
| Badge (count, mention), unread dot | `.ms-badge*`, `.ms-dot` | `w-badge` | DuVay class | F2 |
| Tag (Chat, Email, OK, warning, danger) | `.ms-tag*` | `w-chip` (small, tonal) | `components/app/tag/tac.html` `<app-tag>` | F1 |
| Avatar (sizes, external, channel) | `.ms-avatar*` | `w-avatar` | `components/app/avatar/tac.html` `<app-avatar>` | F1 |
| Presence mark (active, idle, dnd, offline) | `.ms-presence*` | `presence-dots.css` | `components/app/presence/tac.html` `<app-presence>` | F4 |
| Menu and popover (radio items, disabled with reason, danger) | `.ms-popover`, `.ms-menu*` | `w-menu`, `w-popover` | `components/app/menu/tac.html` `<app-menu>` | F1 |
| Tooltip | `.ms-tooltip` | `w-tooltip` | DuVay class | all |
| Dialog and bottom sheet | `.ms-scrim`, `.ms-dialog*`, `.ms-scrim--sheet` | `w-dialog`, `w-bottom-sheet` | `components/app/dialog/tac.html` `<app-dialog>` | F1 |
| Toast | `.ms-toast-region`, `.ms-toast` | `w-snackbar` | `components/app/toasts/tac.html` `<app-toasts>` + `ToastService.go` | F1 |
| Banner (info, email, warning, error) | `.ms-banner*` | `w-banner` | `components/app/banner/tac.html` `<app-banner>` | F1 |
| Skeleton, spinner, progress | `.ms-skeleton*`, `.ms-spinner`, `.ms-progress` | `w-skeleton-loader`, `w-progress-circular`, `w-progress-linear` | DuVay classes | all |
| Empty and error state | `.ms-empty*` | `w-empty-state` | `components/app/empty/tac.html` `<app-empty>` | F1 |
| Tabs | `.ms-tabs`, `.ms-tab` | `w-tabs` | DuVay classes | F4, settings |
| Card, row list, table | `.ms-card*`, `.ms-row*`, `.ms-table` | `w-card`, `w-list`, `w-table` | DuVay classes | settings, F5 |
| Keyboard key | `kbd`, `.ms-kbd` | `w-kbd` | DuVay class | all |

## App shell and navigation (F1, F2)

| Component | Mockup class | Mockups | Tac target | Slice |
| --- | --- | --- | --- | --- |
| App shell (rail, sidebar, main, thread, bottom nav; one pane under 768 px) | `.ms-app`, `.ms-app--thread`, `data-pane` | `messenger.html` | `components/app/shell/tac.html` `<app-shell>` + `ShellService.go` (current pane, current domain) | F1 |
| Skip links and live announcer | `.ms-skip-link`, `#announcer` | `messenger.html` | inside `<app-shell>`; `AnnounceService.go` | F1 |
| Domain rail and domain tile | `.ms-rail`, `.ms-domain-tile` | `messenger.html`, `domain-switcher.html` | `components/app/rail/tac.html` `<app-rail>` + `DomainService.go` | F1 |
| Domain menu (switch, sign in, sign out) and its phone sheet | `.ms-domain-name` + `.ms-menu` | `domain-switcher.html` | `components/app/domain-menu/tac.html` `<app-domain-menu>` | F1 |
| Sidebar (head, search button, sections, footer) | `.ms-sidebar*` | `messenger.html` | `components/sidebar/nav/tac.html` `<sidebar-nav>` + `SidebarService.go` | F2 |
| Sidebar section (collapsible, add button) | `.ms-nav-section*` | `messenger.html` | `components/sidebar/section/tac.html` `<sidebar-section>` | F2 |
| Sidebar row (channel, DM, group DM, email conversation; current, unread, mention, muted) | `.ms-nav-item*` | `messenger.html` | `components/sidebar/item/tac.html` `<sidebar-item>` | F2 |
| Me button (avatar, presence, address) | `.ms-me` | `messenger.html`, `presence-menu.html` | `components/sidebar/me/tac.html` `<sidebar-me>` | F2 |
| Bottom navigation (phones) | `.ms-bottom-nav*` | `messenger.html`, `mobile-home.html` | `components/app/bottom-nav/tac.html` `<app-bottom-nav>` | F1 |

## Conversation (F2)

| Component | Mockup class | Mockups | Tac target |
| --- | --- | --- | --- |
| Conversation view (header + timeline + composer slot) | `.ms-main` | `messenger.html` | `components/conversation/view/tac.html` `<conversation-view>` + `ConversationService.go` |
| Conversation header (title, topic, members, actions, back) | `.ms-header*`, `.ms-members-btn` | `messenger.html` | `components/conversation/header/tac.html` `<conversation-header>` |
| Timeline (`role="log"`, day divider, New divider, jump to new) | `.ms-timeline`, `.ms-day`, `.ms-new-divider` | `messenger.html`, `states-error.html` | `components/conversation/timeline/tac.html` `<conversation-timeline>` + `RealtimeService.go` |
| Message (grouped, mention, highlight, system, pending, failed, deleted, edited) | `.ms-msg*` | `messenger.html`, `delivery-status.html`, `message-delete.html` | `components/conversation/message/tac.html` `<conversation-message>` |
| Message actions toolbar and phone action sheet | `.ms-msg__actions` | `messenger.html`, `mobile-conversation.html` | `components/conversation/actions/tac.html` `<conversation-actions>` |
| Reactions | `.ms-reactions`, `.ms-reaction` | `messenger.html` | `components/conversation/reactions/tac.html` `<conversation-reactions>` |
| Thread summary link | `.ms-thread-link` | `messenger.html` | part of `<conversation-message>` |
| Attachment (file card) | `.ms-file*` | `messenger.html`, `email-conversation.html` | `components/conversation/file/tac.html` `<conversation-file>` |
| Delivery line and delivery details | `.ms-delivery*` | `delivery-status.html` | `components/conversation/delivery/tac.html` `<conversation-delivery>` |
| Email card (subject, From/To/Cc, sender check, quoted text, remote images) | `.ms-email-card*` | `email-conversation.html`, `email-inbound-new.html` | `components/conversation/email-card/tac.html` `<conversation-email-card>` |
| Banners in a conversation (outside people, archived, offline) | `.ms-banner*` | `email-conversation.html`, `channel-details.html`, `states-error.html` | `<app-banner>` |

## Composer (F3)

| Component | Mockup class | Mockups | Tac target |
| --- | --- | --- | --- |
| Composer (chat mode, email mode, reply mode) | `.ms-composer*` | `messenger.html`, `compose-*.html` | `components/composer/box/tac.html` `<composer-box>` + `ComposerService.go` |
| Recipient row (To, Cc, Bcc) with input and suggestions | `.ms-composer__row`, `.ms-recipients`, `.ms-suggest*` | `compose-new.html` | `components/composer/recipients/tac.html` `<composer-recipients>` + `RecipientService.go` (calls `/api/v1/recipients/resolve`) |
| Recipient chip (chat, email, both, resolving, invalid, warning) | `.ms-chip*` | `compose-*.html` | `components/composer/chip/tac.html` `<composer-chip>` |
| Chip delivery menu (Chat / Email / Chat and email, disabled reason) | `.ms-menu` with `menuitemradio` | `compose-chip-menu.html` | `components/composer/chip-menu/tac.html` `<composer-chip-menu>` |
| Email fields (From, Subject, Cc/Bcc toggles) and email mode bar | `.ms-composer__mode--email`, rows | `compose-email.html`, `compose-mixed.html` | `components/composer/email-fields/tac.html` `<composer-email-fields>` |
| Send summary (who gets what; error variant) | `.ms-composer__summary*` | `compose-mixed.html`, `compose-errors.html` | `components/composer/summary/tac.html` `<composer-summary>` |
| Attachment chips with progress | `.ms-attach-list`, `.ms-attach-chip` | `compose-email.html` | `components/composer/attachments/tac.html` `<composer-attachments>` + `UploadService.go` |
| Signature block | `.ms-signature` | `compose-email.html` | part of `<composer-email-fields>` |
| Send button (chat, email, mixed label, disabled) | `.ms-send*` | `compose-*.html` | part of `<composer-box>` |
| Recipients-changed dialog (409) | `.ms-dialog` | `compose-errors.html` | `components/composer/changed/tac.html` `<composer-changed>` |

## Threads, search, activity, presence (F4)

| Component | Mockups | Tac target |
| --- | --- | --- |
| Thread pane (root, replies divider, composer with "Also send to #channel") | `messenger.html` | `components/thread/pane/tac.html` `<thread-pane>` + `ThreadService.go` |
| Search palette (combobox + listbox, recent searches) | `search-palette.html` | `components/search/palette/tac.html` `<search-palette>` + `SearchService.go` |
| Search page, filter bar, result row | `search.html`, `search-states.html` | `components/search/panel/tac.html` `<search-panel>`, `components/search/filter-bar/tac.html`, `components/search/result/tac.html` |
| Activity list, tabs, notification row | `activity.html` | `components/notifications/list/tac.html` `<notifications-list>`, `components/notifications/item/tac.html` + `NotificationService.go` |
| Presence and account menu (do not disturb, status message, sign out) | `presence-menu.html` | `components/notifications/presence-menu/tac.html` + `PresenceService.go` |
| Profile card (same domain: Message + Email; other domain or outside: email only) | `profile-card.html`, `cross-domain.html` | `components/app/profile-card/tac.html` `<app-profile-card>` (F1 owns it, F2-F4 open it) |

## Email-specific parts (F2 and F3)

| Component | Mockups | Tac target |
| --- | --- | --- |
| Participants bar (who is in, and how: Chat or Email) | `email-conversation.html` | `components/conversation/participants/tac.html` |
| Sender check (SPF, DKIM, DMARC; verified or warning, with details) | `email-conversation.html`, `email-inbound-new.html` | `components/conversation/email-auth/tac.html` |
| Hidden images and unverified-sender notice | `email-inbound-new.html` | `components/conversation/email-notice/tac.html` |
| "Only people in example.com see this" marker | `email-conversation.html` | part of `<conversation-message>` |
| Show original dialog (headers, plain text, download .eml) | `email-original.html` | `components/conversation/original/tac.html` + `EmailService.go` |
| Reply mode bar (Reply all / Reply to one; collapsed subject) | `compose-reply-email.html`, `email-conversation.html` | `components/composer/reply-mode/tac.html` |
| "Send to customer" choice in a channel thread from email | `channel-email.html` | `components/composer/send-to/tac.html` |
| Domain explainer and "How domains work" dialog | `cross-domain.html`, `compose-mixed.html` | `components/app/domain-explainer/tac.html` (F1) |

## Delivery and errors (F2, F3)

| Component | Mockups | Tac target |
| --- | --- | --- |
| Delivery details popover (each recipient, Chat or Email, status, times) | `delivery-status.html` | `components/conversation/delivery-details/tac.html` + `DeliveryService.go` |
| Bounce or failure notice on a message, with actions | `delivery-status.html` | `components/conversation/notice/tac.html` |
| Failed chat send row (Retry, Delete; same `clientMessageId`) | `delivery-status.html`, `states-error.html` | `components/conversation/retry/tac.html` |
| Composer note (error, warning, info inside the composer) | `compose-errors.html` | `components/composer/note/tac.html` |
| Attachment size meter | `compose-errors.html` | `components/composer/attach-meter/tac.html` |
| Delete confirm with the "email cannot be recalled" warning | `message-delete.html` | `<app-dialog>` used by `<conversation-actions>` |
| Connection banner (offline, reconnecting) and jump-to-new pill | `states-error.html`, `session-expired.html` | `components/app/connection/tac.html` + `ConnectionService.go`; `components/conversation/jump-pill/tac.html` |

## Channels (F2; channel admin in F5)

| Component | Mockups | Tac target |
| --- | --- | --- |
| Channel browser and row (Join, Joined, Joining) | `channel-browse.html` | `components/sidebar/channel-browser/tac.html`, `components/sidebar/channel-row/tac.html` |
| Create channel dialog (name rules, Public or Private, add people) | `channel-create.html` | `components/sidebar/channel-create/tac.html` + `ChannelService.go` |
| Channel details panel (about, email address, members, notifications, leave) | `channel-details.html` | `components/thread/channel-details/tac.html` (uses the thread pane slot) |
| Archived notice in place of the composer | `channel-details.html` | `components/conversation/readonly-notice/tac.html` |

## Sign-in and session (F1)

| Component | Mockups | Tac target |
| --- | --- | --- |
| Sign-in pages (email, domain found, redirecting, errors, signed out) | `signin*.html`, `signed-out.html` | `pages/signin/tac.html`, `pages/signin/callback/tac.html`, `pages/signed-out/tac.html` + `SessionService.go` (OIDC with PKCE, `login_hint`) |
| Wordmark | sign-in pages | `components/app/wordmark/tac.html` |
| Domain row (recent domain with address) | `signin.html`, `signed-out.html` | `components/app/domain-row/tac.html` |
| Session ended dialog (draft kept) | `session-expired.html` | `components/app/session-ended/tac.html` |

## Settings (new group: no slice owns it yet)

| Component | Mockups | Tac target |
| --- | --- | --- |
| Settings page frame (nav, top tabs under 1100 px, save status) | `settings*.html` | `pages/settings/tac.html` + `SettingsService.go` |
| Save status (one live region: Saving, Saved, Not saved) | `settings*.html` | `components/settings/save-status/tac.html` |
| Settings row, radio cards, segmented choice | `settings*.html`, `channel-create.html` | `components/settings/row/tac.html`, `components/app/choice/tac.html`, `components/app/segmented/tac.html` |
| Push permission panel (not asked, on, blocked, not supported) | `settings-notifications.html` | `components/settings/push-permission/tac.html` + `PushService.go` |
| Quiet hours schedule | `settings-notifications.html` | `components/settings/schedule/tac.html` |
| Signature editor, send-from addresses | `settings-email.html` | `components/settings/signature/tac.html`, `components/settings/send-from/tac.html` |
| Theme picker (DuVay `w-theme`), density, text size, motion | `settings-appearance.html` | `components/settings/theme-picker/tac.html` |
| Keyboard shortcuts list | `settings-appearance.html` | `components/settings/shortcuts/tac.html` |

## Admin app (F5: `apps/web-admin/client/`)

| Component | Mockups | Tac target |
| --- | --- | --- |
| Admin shell and nav | `admin-*.html` | `components/admin/shell/tac.html` (exists), `components/admin/nav/tac.html` |
| Domains list (table on desktop, cards on phones) | `admin-domains.html` | `pages/domains/tac.html`, `components/admin/domain-list/tac.html` + `AdminDomainService.go` |
| Add domain dialog | `admin-domain-add.html` | `components/admin/domain-add/tac.html` |
| DNS records, status marks, Check again, DKIM rotation, danger zone | `admin-domain-dns.html` | `pages/domains/dns/tac.html`, `components/admin/dns-record/tac.html`, `components/admin/dns-check/tac.html`, `components/admin/dkim-rotation/tac.html`, `components/admin/danger-zone/tac.html` |
| Typed confirmation (type the domain name) | `admin-domain-dns.html` | `components/admin/confirm-typed/tac.html` |
| Copy field | `admin-domain-dns.html`, `channel-details.html` | `components/admin/copy-field/tac.html` (web app copy in `components/app/copy-field/`) |
| Members table, role menu, "Manage in Keycloak" | `admin-members.html`, `admin-members-states.html` | `pages/members/tac.html`, `components/admin/member-row/tac.html` + `AdminMemberService.go` |
| No access page (403) | `admin-members-states.html` | `pages/no-access/tac.html` |
| Channel addresses and archive | `admin-channels.html` | `pages/channels/tac.html`, `components/admin/channel-address/tac.html` + `AdminChannelService.go` |
| Mail delivery: queue numbers, suppression list, limits | `admin-mail.html` | `pages/mail/tac.html`, `components/admin/queue-stats/tac.html`, `components/admin/suppression-list/tac.html`, `components/admin/limits-form/tac.html` + `MailOpsService.go` |

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
