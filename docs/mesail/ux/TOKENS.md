# Mesail design tokens

Mesail uses two token layers. The source files are in the mockups folder, and the frontend copies
them unchanged:

1. **DuVay** `--w-*` tokens: [mockups/assets/duvay/tokens.css](mockups/assets/duvay/tokens.css)
   and [mockups/assets/duvay/themes.css](mockups/assets/duvay/themes.css), copied unchanged from
   DuVay (version and commit in `mockups/assets/duvay/SOURCE.txt`). Type scale, spacing, radii,
   shadows, focus ring, motion, breakpoints and every theme colour.
2. **Mesail** `--ms-*` tokens: [mockups/assets/mesail-tokens.css](mockups/assets/mesail-tokens.css).
   Only what DuVay has no name for.

Rule for code: use a `--w-*` token when one says what you mean; use an `--ms-*` token otherwise;
never a raw value. Themes switch with DuVay's `w-theme` attribute on `<html>`: `light`, `dark`,
`auto` (follows the system) and `high-contrast`. Every `--ms-*` colour has a value for each.

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
| `--ms-row-height` | 2rem | sidebar rows with a pointer |
| `--ms-touch-min` | 2.75rem (44 px) | every target on touch screens and under 768 px |

Breakpoints (from DuVay's scale): **768 px** (one pane below it) and **1200 px** (thread pane
beside the conversation at and above it).

### Avatars and tiles
`--ms-avatar-xs` 20 px, `-sm` 24, `-md` 36 (messages), `-lg` 48, `-xl` 80 (profile);
`--ms-domain-tile` 44 px. Avatars are rounded squares; the colour comes from a per-person hue
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
| `--ms-radius-chip` / `-card` / `-tile` | pill / 8 px / 12 px |
| `--ms-duration` / `--ms-duration-panel` | 120 ms / 200 ms, `--ms-ease` standard curve; all off with reduced motion |

## Checked contrast (WCAG 2.2 AA)

Computed with the WCAG formula on 2026-11-10: email text 7.0:1 (light) and 8.7:1 (dark) on the
surface; email labels 10.6:1 / 9.3:1 / 14.0:1; chat labels 10.8:1; rail text 12.6:1; subtle text on
the sidebar 5.8:1; mention badges 6.5:1. DuVay's own pairs are documented in DuVay.
