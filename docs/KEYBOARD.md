# Keyboard Commands

CADUCEUS provides a context-aware, user-customizable command system based on
the Mailspring keymaps pinned in `docs/mailspring-parity.json`. The versioned
catalog is `client/shared/data/keymap.json`; the browser and authenticated
preferences API validate the same command IDs, binding grammar, scopes, and
conflict rules.

Press `?` outside an editor to open the accessible command dialog. The dialog
shows the effective bindings and restores focus when closed. Use **Settings →
Keyboard shortcuts** to customize them. A blank field disables that command;
**Restore defaults** removes all overrides.

## Binding grammar

- One key: `c`, `/`, `?`, `enter`, `left`
- One modifier chord: `mod+enter`, `shift+r`
- A two-step sequence: `g i`, `* a`
- `mod` means Command on Apple platforms and Control elsewhere.
- Commands are suppressed in inputs, textareas, selects, and editable content,
  except the explicitly allowed composer Send command.
- The server rejects unknown commands, malformed bindings, and collisions in
  contexts that can be active together. Mutually exclusive contexts may share
  a binding, such as `shift+r` for mailbox refresh and reader Reply All.

## General and navigation

| Shortcut | Context | Action |
|---|---|---|
| `?` | Global | Show command dialog |
| `c` | Global | Compose new message |
| `/` | Mailbox | Focus search |
| `shift+r` | Mailbox | Refresh mailbox |
| `mod+shift+p` | Global | Toggle PII-safe screenshot mode |
| `j` / `k` | Mailbox | Select next / previous message |
| `d` / `u` | Mailbox | Scroll down / up |
| `enter` | Mailbox | Open selected message |
| `* a` / `esc` | Mailbox | Select all / clear selection |
| `g i`, `g d`, `g s` | Global | Go to Inbox, Drafts, or Sent |
| `g a`, `g x`, `g t` | Global | Go to Archive, Spam, or Trash |
| `g ,` | Global | Go to Settings |

## Message, reader, and composer

| Shortcut | Context | Action |
|---|---|---|
| `s`, `e`, `#`, `!` | Mail | Star, archive, trash, or report spam |
| `shift+i` / `shift+u` | Mail | Mark read / unread |
| `r`, `shift+r`, `f` | Reader | Reply, Reply All, or forward |
| `mod+p` | Reader | Print the conversation |
| `mod+f` | Reader | Find in conversation |
| `mod+g` / `mod+shift+g` | Reader find | Next / previous match |
| `mod+g` | Reader without find | Open GitHub notification target |
| `mod+enter` | Composer | Send |
| `mod+shift+t` | Composer | Focus To |
| `mod+shift+c` | Composer | Focus CC |
| `mod+shift+b` | Composer | Focus BCC |
| `mod+shift+f` | Composer | Focus From |
| `mod+b`, `mod+i`, `mod+u` | Composer body | Bold, italic, or underline |
| `mod+k` | Composer body | Insert a safe link |
| `mod+shift+7`, `mod+shift+8` | Composer body | Numbered or bullet list |
| `esc` | Composer | Discard draft |

## Calendar

The complete upstream calendar command surface is supported. An owned event
must have keyboard focus before a move or resize command can mutate it.

| Shortcut | Action |
|---|---|
| Arrow keys | Move the selected event; vertical movement is 15 minutes in timed views and horizontal movement is one calendar day |
| `shift` + Arrow keys | Resize the selected event using the same increments |
| `mod+1`, `mod+2`, `mod+3`, `mod+4` | Day, Week, Month, or Agenda view |
| `mod+t` | Go to today |
| `mod+left` / `mod+right` | Previous / next calendar range |
| `mod+shift+r` | Refresh calendars |

Month and Agenda views intentionally ignore vertical move/resize commands,
matching Mailspring. Calendar writes remain owner-scoped by the API.

## Persistence and safety

Overrides are stored per authenticated user in the FYLO-backed users schema
(`v14`). The API normalizes aliases, removes values equal to defaults, accepts
`off` for disabled commands, and applies updates atomically. Invalid historical
values are dropped on read rather than weakening the current catalog. Browser
and server tests cover discovery, focus restoration, input suppression,
context-exclusive duplicate bindings, conflict rejection, reload persistence,
and all calendar command classes in Chrome, Firefox, and WebKit.
