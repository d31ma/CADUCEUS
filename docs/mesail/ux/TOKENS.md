# Mesail design tokens

Mesail uses two token layers, plus a Tailwind theme that points at them:

1. **DuVay** `--w-*` tokens, pinned to DuVay **v26.35.01-1** (github `d31ma/DUVAY`, commit `bc8b56a`):
   `src/tokens.css` and `src/themes.css`, shipped inside Mesail's DuVay CSS subset
   (`mockups/assets/vendor/duvay-mesail.css`, built by `mockups/build/build.sh`). Type scale, spacing,
   radii, shadows, focus ring, motion, breakpoints, touch target and every theme colour. They are the
   source of truth.
2. **Mesail** `--ms-*` tokens: [mockups/assets/mesail-tokens.css](mockups/assets/mesail-tokens.css).
   Only what DuVay has no name for.
3. **Tailwind theme** ([mockups/build/tailwind.input.css](mockups/build/tailwind.input.css), Tailwind CSS
   4.3.5): breakpoints, colours, fonts, radii and text sizes are DuVay or Mesail tokens (`@theme inline`,
   so a theme switch repaints utilities too). Tailwind's default palette and breakpoints are removed.

Rule for code: use a `--w-*` token when one says what you mean; use an `--ms-*` token otherwise;
never a raw value. In markup, use the Tailwind names below. Themes switch with DuVay's `w-theme`
attribute on `<html>`: `light`, `dark`, `auto` (follows the system) and `high-contrast`. Every `--ms-*`
colour has a value for each. Density (`w-density`) and platform skin (`w-os`) are DuVay's too.

## Breakpoints

Decided on 2026-11-11 (the user's choice "Keep 768 px"): the apps have ONE layout step, at 768 px. There is
no separate 600 px or 960 px layout. The table matches the code in `apps/web` and `apps/web-admin`.

| Tailwind | Width | Mesail layout |
| --- | --- | --- |
| (base), `max-[768px]:` | under 768 px | phones and narrow windows: one pane at a time, bottom navigation, 44 px targets |
| `min-[768px]:` | 768 px | domain rail + sidebar + conversation; a thread or details pane covers the conversation |
| `lg` | 1280 px | the thread or details pane docks as a fourth column |
| `xl` | 1920 px | wider panes: rail 4.5rem, sidebar 19rem, thread 28rem |
| `2xl` | 2560 px | wider panes (sidebar 21rem, thread 34rem), UI scaled to 112.5 % |
| `short:` | height under 520 px | landscape phones: 44 px headers, no hint lines |
| `pointer-coarse:`, `touch:` | touch screens | 44 px targets |

Inside a component, Tailwind's `sm` (600 px) still sets card padding (`--ms-card-pad`), and a few dense rows
switch at 960 px (the one-line delivery status in a conversation). These change one component, not the page
layout.
The mockups in this folder still show DuVay's 600 and 960 px layout steps; the apps follow this table.

Containers (for `@container` queries on panes and cards) use Tailwind's `@xs` 20rem to `@3xl` 64rem plus
its larger defaults.

## Tailwind names for tokens

| Tailwind utility root | Token |
| --- | --- |
| `surface`, `surface-low`, `surface-mid`, `surface-high`, `sidebar` | `--w-surface`, `--w-surface-container-low`, `-container`, `-container-high`, `--w-sidebar` |
| `text`, `subtle`, `very-subtle` | `--w-text`, `--w-text-subtle`, `--w-text-very-subtle` |
| `divider`, `border`, `outline` | `--w-divider`, `--w-border`, `--w-outline` |
| `hover`, `selected`, `on-selected` | `--w-hover`, `--w-selected`, `--w-selected-text` |
| `accent`, `accent-bg`, `on-accent`, `primary-container`, `on-primary-container` | DuVay accent and primary pairs |
| `error`, `error-container`, `warning-container`, `success-container` (+ `on-…`) | DuVay feedback pairs |
| `chat`, `chat-bg`, `on-chat-bg`, `email`, `email-bg`, `on-email-bg`, `email-rule` | Mesail delivery colours |
| `rail`, `on-rail`, `msg-hover`, `msg-highlight` | Mesail rail and message colours |
| `font-sans`, `font-mono` | `--w-font-family`, `--w-font-family-mono` |
| `rounded-sm|md|lg|xl` | `--w-radius`, `-md`, `-lg`, `-xl` |
| `text-xs|sm|base|lg|xl|2xl` | `--w-font-xs` (12), `-base` (14), `-md` (15), `-lg` (17), `-xl` (20), `-2xl` (24) |
| spacing (`p-4`, `gap-2`, …) | Tailwind's 0.25rem step, the same 4 px grid as `--w-space-*` |

Mesail utilities: `safe-t`, `safe-b`, `safe-l`, `safe-r` (Tac title-bar insets, else `env(safe-area-inset-*)`),
`touch-target` (44 px), `drag-region` / `no-drag` (window drag areas).

## DuVay tokens Mesail relies on

| Role | Tokens |
| --- | --- |
| Surfaces | `--w-surface`, `--w-surface-container-low`, `--w-surface-container`, `--w-surface-container-high`, `--w-sidebar`, `--w-toolbar` |
| Text | `--w-text`, `--w-text-subtle`, `--w-text-very-subtle` |
| Accent | `--w-accent` (text), `--w-accent-bg` + `--w-on-accent` (fills), `--w-primary-container` + `--w-on-primary-container` |
| Lines | `--w-divider`, `--w-border` (decorative), `--w-outline` (control edges, 3:1) |
| States | `--w-hover`, `--w-selected`, `--w-selected-text` |
| Feedback | `--w-error*`, `--w-success*`, `--w-warning*` (each with `-container` and `on-` pairs) |
| Overlays | `--w-scrim`, `--w-inverse-surface`, `--w-inverse-on-surface`, `--w-shadow-md` |
| Type | `--w-font-family`, `--w-font-family-mono`, `--w-font-xs` (12) … `--w-font-2xl` (24), weights |
| Space | `--w-space-1` (4 px) … `--w-space-10`, 4 px steps |
| Shape | `--w-radius` (4), `-md` (6), `-lg` (8), `-xl` (12), `-pill` |
| Focus | `--w-focus-ring-width` (2 px), `--w-focus-ring-color`, `--w-focus-ring-offset` |

## Mesail tokens

### Layout
| Token | Value | Use |
| --- | --- | --- |
| `--ms-rail-width` | 4.25rem (68 px) | domain rail |
| `--ms-sidebar-width` | 17rem (272 px) | conversation sidebar |
| `--ms-thread-width` | 25rem (400 px) | thread and details pane |
| `--ms-header-height` | 3.25rem (52 px) | pane headers (plus top safe area on phones) |
| `--ms-bottom-nav-height` | 3.5rem (56 px) | phone bottom navigation (plus bottom safe area) |
| `--ms-content-max` | 50rem | settings and admin pages |
| `--ms-dialog-max` | 32rem | dialogs |
| `--ms-row-height` | 2rem (32 px) | sidebar rows and admin nav rows with a pointer; 44 px on touch screens and under 768 px (decided 2026-11-11) |
| `--ms-touch-min` | 2.75rem (44 px) | alias of DuVay `--w-touch-min`; every target on touch screens |

Breakpoints: see the table above (768, 1280, 1920 and 2560 px).

### Control sizes (one scale, since 2026-11-11)
Every control has one of three heights, so buttons, fields, menu rows and icon buttons line up.
| Token | Value | Tailwind | Use |
| --- | --- | --- | --- |
| `--ms-control-sm` | 2rem (32 px) | `h-control-sm`, `min-h-control-sm` | menu-nav rows, small buttons, checkbox rows (pointer only) |
| `--ms-control-md` | 2.5rem (40 px) | `h-control-md`, `min-h-control-md` | default: buttons, text fields (DuVay's field is 40 px), selects, menu items |
| `--ms-control-lg` | 3rem (48 px) | `min-h-control-lg` | the main action of a card or dialog (Sign in, Save and continue) |
| `--ms-touch-min` | 2.75rem (44 px) | `min-h-touch`, `size-touch` | floor for every target on touch screens and in windows under 768 px |
| `--ms-icon-sm` / `-md` / `-lg` / `-xl` | 1 / 1.25 / 1.5 / 2rem | `size-icon-sm` ... | icons inside text and controls / standalone and navigation / large / empty-state tiles |
| `--ms-radius-control` / `-dialog` | `--w-radius-lg` (8) / `--w-radius-xl` (12) | `rounded-lg` / `rounded-xl` | controls: buttons of every size, fields, selects / cards and dialogs |
| `--ms-nav-pill-width` | 3.5rem (56 px) | | the selected pill behind a bottom-navigation icon |
| `--ms-auth-card-max` | 26rem (416 px) | `max-w-auth` | sign-in and password cards |
| `--ms-dialog-max`, `--ms-content-max` | 32rem, 50rem | `max-w-dialog`, `max-w-content` | dialogs, settings and admin pages |
| `--ms-card-pad` / `-narrow` | 1.5rem / 1rem | `sm:p-6` / `p-4` | card padding from 600 px / under it |
| `--ms-space-field`, `--ms-space-label`, `--ms-space-section` | 1rem, 0.375rem, 1.5rem | `gap-4`, `gap-1.5`, `gap-6` | between fields, label to field, between sections |

**Buttons and fields share one corner** (decided 2026-11-11, the user's choice "Both 8 px"): every DuVay button,
small ones included, has 8 px corners like the fields. DuVay draws `.w-btn` corners from `--w-radius` (4 px) and
renders every button as a native `<button>` or `<a>`, so `shared/styles/app.css` sets
`button, a { --w-radius: var(--ms-radius-control); }`. Other clickable DuVay controls drawn from those elements
(menu items, alert close buttons) get 8 px too. Surfaces that use `--w-radius` (tooltips, snackbars, skeletons)
keep 4 px. TY1403 allows only plain HTML elements in app.css, so the custom element `w-btn` cannot carry the rule.

**Icon buttons are square at the control height of their row:** 40 px next to 40 px fields and buttons.
A dense row (sidebar rows, the remove button on a chip, row actions in a list) uses `--ms-control-sm` (32 px)
with a mouse; under `touch:` it grows to `--ms-touch-min` (44 px) like every other target. Agreed with slice F3,
which adds the same tokens with the same values to `apps/web` and `apps/web-admin` `shared/styles/app.css`.

The Tailwind variant `touch:` (pointer: coarse, or a window under 768 px) matches the rule of `tests/responsive`.
`mesail-components.css` applies the floor to DuVay's own controls in that case (`.w-btn`, `.w-icon-btn`,
`.w-checkbox`, `.w-text-field-control`, `.w-select-field`, and since slice OU `.w-chip`, `.w-list-item`,
`.w-tab` (width), the recipient input and the thread link), so a small DuVay control grows to 44 px on a phone.
Tabs never shrink (`flex: none`): a tab strip scrolls sideways instead of cutting labels.

The icon classes `.ms-icon-xs` / `-sm` are not Tailwind utilities: since the `--spacing-icon-*` names exist,
Tailwind read `ms-icon-sm` as "margin-inline-start: icon-sm" and moved every small icon 16 px to the right.
`tailwind.input.css` now excludes those class names (`@source not inline(...)`). Apps do not define
`--spacing-icon-*`, so they are not affected.

Offline states (slice OU, `screens/offline.css`) add no colour token: the sync bar uses
`--w-surface-container-high` (offline), the primary, warning and success container pairs (updating, needs
attention, up to date); waiting items use `--w-on-warning-container` text and a `--w-warning-container` edge;
the space meter uses `--w-primary`, `--ms-email`, `--ms-presence-idle` and `--w-text-very-subtle`, always with a
labelled list beside it.

Browser push states (slice N2, `push-states.html`) add no token either: status lines use the inverse pair
(`bg-inverse` / `text-on-inverse`, DuVay's snackbar colours), errors the error-container pair, and the drawn
system notification `--w-surface-container-high` with the `shadow-xl` utility, like dialogs.
Passkeys and the app lock (slice PU, README section 17) add no token. Failure lines reuse the notice pairs
(`bg-surface-high` for "nothing happened", the warning and error container pairs); the locked screen uses the
auth card (`max-w-auth`) with a `--w-primary-container` lock tile. One icon is new: `fingerprint` (three arcs,
the same 2 px stroke on the 24 px grid) for every passkey button, the Passkeys row and the unlock button.

Proportions: a card is never wider than its content needs (auth 26rem, dialog 32rem), its main button fills
the card width at 48 px, and secondary buttons sit at 40 px, so the main action is always the largest.

### Avatars and tiles
`--ms-avatar-xs` 20 px, `-sm` 24, `-md` 36 (messages), `-lg` 48, `-xl` 80 (profile);
`--ms-domain-tile` 44 px. Avatars are rounded squares (`--ms-avatar-radius`, = `--w-radius-lg`, 8 px); the colour comes from a per-person hue
(`--h`), light and dark variants are computed in CSS.

### Delivery colours
| Token | Light | Dark | High contrast | Use |
| --- | --- | --- | --- | --- |
| `--ms-chat` | = `--w-primary` | = `--w-primary` | = `--w-primary` | chat icon colour |
| `--ms-chat-bg` / `--ms-on-chat-bg` | = `--w-primary-container` pair | same | same | "Chat" label on chips and tags (10.8:1 light) |
| `--ms-email` | `#6b3fa0` | `#cdb4f2` | `#3b1366` | email icons, Send email button fill (7.0:1 / 8.7:1 / 14.3:1 on surface) |
| `--ms-email-bg` / `--ms-on-email-bg` | `#efe7f8` / `#3f2366` | 16% violet / `#efe4fc` | `#e9dcf7` / `#1e0838` | "Email" labels, email mode bar (10.6:1 / 9.3:1 / 14.0:1) |
| `--ms-email-rule` | `#9a72cc` | `#a688d4` | `#3b1366` | left rule of email cards, email composer border |

Violet was chosen so email never reads as an error (red), a warning (amber) or chat (teal).

### Presence
| Token | Light | Dark | Shape |
| --- | --- | --- | --- |
| `--ms-presence-active` | `#2e7d32` (4.9:1) | `#66bb6a` | filled dot |
| `--ms-presence-idle` | `#b26a00` (4.0:1) | `#f2b84b` | crescent |
| `--ms-presence-dnd` | `#b3261e` (6.2:1) | `#f2817a` | dot with a bar |
| `--ms-presence-offline` | `#7b878c` (3.5:1) | `#9aa5a9` | ring |

All meet 3:1 against the surface (WCAG 1.4.11), and the shapes differ, so colour is never the only
cue. Screen readers get the state as text ("Bo Lee, active").

### Counts, rail, messages
| Token | Value | Use |
| --- | --- | --- |
| `--ms-mention-bg` / `--ms-on-mention-bg` | = `--w-error` pair (6.5:1) | mention count badges |
| `--ms-unread-dot` | = `--w-primary` | unread dot |
| `--ms-rail-bg` / `--ms-on-rail` | `#12313d` / `#e8f8fc` (12.6:1); dark `#0d1416` | domain rail |
| `--ms-rail-tile`, `--ms-rail-tile-active`, `--ms-on-rail-tile-active` | translucent / white / `#12313d` (13.7:1) | domain tiles |
| `--ms-msg-hover` | = `--w-surface-container-low` | message row hover and focus |
| `--ms-msg-highlight` | = `--w-warning-container` | message that mentions you |
| `--ms-new-divider` | = `--w-error` | "New" line at the read position |
| `--ms-code-bg`, `--ms-quote-rule` | container high, border | inline code, quotes |

### Type roles, shape and motion
| Token | Value |
| --- | --- |
| `--ms-text-body` | `--w-font-md` (15 px), line height `--ms-line-body` 1.5 |
| `--ms-text-meta` | `--w-font-sm` (12 px) |
| `--ms-text-nav` | `--w-font-base` (14 px) |
| `--ms-text-title` / `--ms-text-heading` | `--w-font-lg` (17) / `--w-font-xl` (20) |
| `--ms-radius-chip` / `-card` / `-tile` | pill / 12 px (`--w-radius-xl`) / 12 px |
| `--ms-duration` / `--ms-duration-panel` | 120 ms / 200 ms, `--ms-ease` standard curve; all off with reduced motion |

## Checked contrast (WCAG 2.2 AA)

Computed with the WCAG formula on 2026-11-10: email text 7.0:1 (light) and 8.7:1 (dark) on the
surface; email labels 10.6:1 / 9.3:1 / 14.0:1; chat labels 10.8:1; rail text 12.6:1; subtle text on
the sidebar 5.8:1; mention badges 6.5:1. DuVay's own pairs are documented in DuVay.
