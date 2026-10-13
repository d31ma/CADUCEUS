# Trusted extensions

CADUCEUS can sideload per-mailbox plugins and declarative themes from
**Settings → Extensions**. Packages are local JSON files; there is no package
registry, hosted dependency, update service, or administrator-wide implicit
trust.

Selecting a file only inspects it. CADUCEUS normalizes the package and shows
its permissions and SHA-256 digest. **Trust exact digest and install** is the
separate authority boundary. Installation starts disabled, and a later file
with the same package ID must be reviewed as a new digest.

## Plugin package

```json
{
  "manifest": {
    "schemaVersion": 1,
    "id": "example.follow-up",
    "displayName": "Follow-up helper",
    "description": "Adds a local toolbar command.",
    "version": "1.0.0",
    "kind": "plugin",
    "engine": { "caduceus": "26" },
    "permissions": ["mail:metadata", "ui:notify"],
    "contributions": [
      { "id": "remind", "label": "Remind me", "slot": "message-toolbar" }
    ]
  },
  "source": "caduceus.activate({ onCommand(command, context) { return { notify: `${command.id}: ${context.mail.subject}` } } })"
}
```

The source must call `caduceus.activate({ onCommand })` exactly once. The
handler receives a frozen `{ id, slot }` command plus only the host context
covered by its declared permissions. It may return a local `notify` string or
a plain-text `composePatch` covered by its declared authority. Async handlers
are supported. Commands have a two-second deadline.

| Permission | Context or result |
| --- | --- |
| `mail:metadata` | Current message ID, subject, sender, and received time |
| `compose:read` | Current To, Cc, Bcc, subject, and plain-text draft |
| `compose:write` | Plain-text To, Cc, Bcc, subject, or draft patch |
| `ui:notify` | Bounded local toast text |

Contribution slots are `app-toolbar`, `composer-toolbar`, and
`message-toolbar`. CADUCEUS creates and styles the buttons; packages cannot
provide markup, styles, icons, or event attributes.

Plugins run in a Web Worker behind an opaque-origin sandboxed iframe. The
sandbox has no application origin, cookies, session/local storage, network,
imports, navigation, forms, popups, fonts, media, or remote assets. Every
cross-frame message is scoped to one activation nonce and permission-filtered
again by the host. Disable, removal, reload, and sign-out terminate the
sandbox.

## Theme package

```json
{
  "manifest": {
    "schemaVersion": 1,
    "id": "example.midnight",
    "displayName": "Midnight",
    "description": "A local dark palette.",
    "version": "1.0.0",
    "kind": "theme",
    "engine": { "caduceus": "26" },
    "permissions": [],
    "theme": {
      "mode": "dark",
      "tokens": {
        "primary": "#88aaff",
        "surface": "#101218",
        "text": "#f5f7ff"
      }
    }
  },
  "source": ""
}
```

Themes contain no executable source, commands, arbitrary CSS, URLs, or remote
assets. Tokens are hexadecimal colors from this fixed catalog:
`danger`, `on-primary`, `primary`, `primary-container`, `surface`,
`surface-container`, `surface-container-high`, `text`, and `text-subtle`.
Exactly one trusted theme can be enabled for a mailbox.

## Operational limits

- package source: 128 KiB;
- contributions per plugin: 12;
- cross-sandbox result: 16 KiB;
- command or activation time: two seconds;
- package ownership: the exact authenticated mailbox, reinforced by FYLO
  UID/GID domain metadata when POSIX access is enabled.

Explicit trust is not a code signature or a security review. Review the
package source and request only the permissions it needs. CADUCEUS deliberately
does not offer a general message-body, DOM, filesystem, shell, network, or
arbitrary navigation permission.
