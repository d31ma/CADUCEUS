# Accessibility qualification

CADUCEUS targets WCAG 2.2 Level AA for the browser application. Accessibility
is a release requirement, not a best-effort enhancement.

## Implemented baseline

- One named document title and one primary `main` landmark per application
  route, with role/name navigation for mailbox search and reader actions.
- Keyboard-operable mailbox rows, filters, dialogs, menus, column separators,
  and the customizable command system.
- Native capture-phase focus containment for every visible modal dialog, plus
  initial-focus and return-focus behavior in the shortcut and contact flows.
- Named status announcements with decorative progress animation hidden from
  assistive technology.
- Visible keyboard focus, reduced-motion support, forced-color focus styling,
  reflow at a 320 CSS-pixel viewport (400% equivalent from 1280 CSS pixels),
  and WCAG text-spacing resilience.
- Light/dark contrast and 24 CSS-pixel minimum target-size enforcement through
  automated WCAG 2.2 rules.

## Automated gate

Run the qualification matrix with:

```sh
bun run test:accessibility
```

The command bundles current source before testing. It runs Chromium, Firefox,
and WebKit against login, a populated message reader, and every authenticated
primary route at phone and desktop sizes. The suite combines axe-core WCAG
2.0/2.1/2.2 A/AA checks with role-driven screen-reader flow assertions,
keyboard focus trapping/restoration, forced colors, reduced motion, WCAG text
spacing, and horizontal-overflow checks.

The gate lives in `verify:phase0:browser`, so pull requests cannot bypass it by
running only the general interaction suite.

## Manual release check

Automated rules cannot replace assistive-technology judgment. Before a stable
release, record these checks in the release evidence:

1. VoiceOver + Safari on macOS: sign in, navigate mailbox landmarks, search,
   open a conversation, reply, and close a modal with focus restored.
2. NVDA + Firefox on Windows: repeat the mailbox/reader flow and verify status
   announcements do not duplicate.
3. VoiceOver on iOS and TalkBack on Android: open the navigation drawer,
   traverse the horizontally scrolling action rows, and compose a message.
4. At 200% and 400% browser zoom, verify no two-dimensional page scrolling is
   required and all controls remain reachable.
5. In operating-system high-contrast/forced-colors and reduced-motion modes,
   verify focus, selection, errors, and disabled states remain distinguishable.

Any failure blocks release or must be recorded as an explicit, owner-assigned
exception with affected flows and a removal date.

