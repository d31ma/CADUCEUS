# ADR 0010: The screens are a tree of page apps

- Status: accepted (2026-11-12)
- Deciders: the owner ("Screens only", option B; no one-server back end)
- Related: ADR 0004 (Tac frontend, Go Yon services), ADR 0006 (offline
  first), ADR 0009 (thin controllers), SLICES.md Wave 8; TACHYON ADR 0086
  ("step 2 of micro-frontends"); the analysis in the coordinator's shared
  notes, `arch-app-tree-answer.md`

## Context

The owner asked whether Mesail's apps can form a tree that follows the
screens: the home page first, then the apps for the pages reachable from
home, and so on, each with its own `client`, `server` and data, built as
one app or as many apps.

Findings (2026-11-12):

- Inside one TACHYON project the pages are already a tree by web address,
  and role folders may nest (TACHYON ADR 0091). But one project is one
  app, with one client, one server and one data source.
- TACHYON cannot yet mount separately built Tac apps under path prefixes
  of one site, or build them into one bundle (TACHYON ADR 0086, "step 2
  of micro-frontends", not built).
- Data does not follow the screens: messages appear in chat, mail, home,
  search and notifications; people in every page. Each piece of data has
  one owner service, and each FYLO root has one writer (ADR 0004 section
  4). A data owner per page would bring the services back under page
  names.

## Decision

1. **The screens become a tree of page apps.** The `web` app and the
   `web-admin` app are split into page apps:

   ```
   home (the shell: sign-in, domain menu, sync bar, notifications)
     chat
     mail
     channels
     search
     settings
     admin (today the separate web-admin app)
   ```

   Each page app has its own `client/`. A page app may have its own small
   `server/` and collection only for data that no other page uses.
2. **One app or many from the same source.** The same page apps build
   either into one bundle (one deploy) or into separate deploys under their
   paths, through TACHYON request R1. Which form an install uses is a
   deploy setting, not a code change.
3. **The web addresses stay the same** in both forms: every current path,
   deep links (notifications open paths, slice N2), the web app manifest
   scope and the native apps' start pages.
4. **The back ends stay services.** `identity`, `directory`, `messaging`,
   `channels`, `mail-in`, `mail-out`, `attachments`, `search`, `notify`
   and `realtime` keep their layout, data and micro-service form. Pages
   call them through the router as today. There is no one-server form of
   the back end (the owner's "No"; it would also serialize every request
   of a domain in Lambda form).
5. **One device copy.** All page apps of one site share one session, one
   service worker and one device store (ADR 0006): the offline copy, the
   queue and the lock belong to the person on the device, not to a page.

## Request to TACHYON

- **R1** (TACHYON ADR 0086 step 2): mount separately built Tac apps under
  path prefixes of one origin (for example `/chat`, `/mail`), and build
  the same Tac apps into one bundle instead, with the same addresses.
  Mesail needs, in both forms:
  - one shell (home) that the other apps render inside, so moving between
    pages does not reload the shell;
  - one session for all apps (the token and the refresh live once);
  - one service worker and one web app manifest for the origin;
  - one TACHYON device `Store` (IndexedDB, T1) shared by every app of the
    origin, with the sealed tables and the lock (T6b, T15);
  - shared DuVay styles and assets loaded once;
  - the per-route size budget still checked per page app;
  - the native shells (T14) load the same apps.

**TACHYON's plan for R1** (approved by the owner, "Accept all", 2026-11-12;
8 slices, started): a page app is mounted with `tac.mount` (slice 1); one
device `Store` is shared (slice 2); one shell stays loaded while pages
change (soft navigation, only on sites with a shell); a new `Session`
client holds the token and its refresh; the shell owns the one service
worker and the web app manifest; the native shells always use the one
bundle; every app of a site must be on the same TACHYON version. Mesail
follows these names: the home app is the shell and owns the service
worker, the manifest and the `Session`; each page app is mounted with
`tac.mount`; the native apps build the one bundle; all page apps move
their TACHYON pin together.

## Consequences

- Wave 8 (SLICES) splits the screens after Wave 7 and after TACHYON R1 is
  released and Mesail's pin moves. It replaces SLICES "Later" X5.
- CONTRACTS, EVENTS and DOMAIN do not change: they describe services.
- Shared client code (the session, the device store, the sync bar, the
  DuVay layer) stays in one place that every page app uses; slice W0
  decides how, from what R1 offers.
- Until R1, nothing moves; new screens keep going into `web` and
  `web-admin` as today, in the page folders they will keep.
