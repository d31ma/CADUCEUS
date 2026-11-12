# ADR 0011: Chat between members goes device to device

- Status: accepted (2026-11-12) as the direction; the details marked
  "open" wait for the owner's answers (section 8)
- Deciders: the owner (option A, with an encrypted backup and the admin
  limits accepted), architecture (the design)
- Supersedes, at the cut-over (section 7): the chat parts of ADR 0004
  (chat stored in `messaging` and `channels` FYLO roots), ADR 0006 (for
  chat, the server is no longer the source of truth), the chat routes and
  events of CONTRACTS ("channels", the chat parts of "messaging",
  "realtime", "notify" and "search") and SPEC's channel rules. Amends ADR
  0007 (a relay server with public ports). Email does not change.
- Related: the analysis in the coordinator's shared notes,
  `arch-p2p-answer.md`; ADR 0008 (passkeys); ADR 0010 (page apps)

## Context

The owner's idea (2026-11-12): chat between people of one domain goes
directly between their devices, the messages live only on the devices and
browsers, and each person's devices sync with each other. Asked why, the
owner chose **privacy: the server must not be able to read chats**. Shown
the options, the owner chose **full peer-to-peer** (servers only for
relays), knowing it was not recommended, with these answers:

- If a person loses all their devices: **an encrypted backup** that they
  unlock; the server stores it but cannot read it.
- Admins can still remove a member's access, but cannot read, keep for
  legal reasons, or delete the chats already on that member's devices:
  **acceptable**.

Facts (2026-11-12): TACHYON's client code reaches browser features only
through `tac.browserApis` (TACHYON ADR 0065): no WebRTC (the browser's
device-to-device connection) and no WebCrypto (the browser's
encryption). Client sockets reach only declared servers (TACHYON ADR
0124). The device copy is TACHYON's IndexedDB `Store` with sealed tables
and the biometric lock (ADR 0006, ADR 0008).

## Decision

1. **Chat messages exist only on devices.** A chat message (direct or in
   a group) is written on the sender's device, encrypted end to end, and
   sent device to device. No server stores chat content in a form it can
   read. Email, its conversations and its files stay on the servers as
   today.
2. **Every device has its own keys.** Each device makes its key pairs
   (X25519 for key agreement, Ed25519 for signing); the private keys never
   leave the device and cannot be exported (TACHYON request P2). The
   public keys are listed in `directory` per person and domain, so the
   other members' devices find them. A new device is added after a full
   sign-in (password and 2-step code, or a passkey) and is announced to
   the person's other devices.
3. **Groups use MLS** (Messaging Layer Security, RFC 9420), also for a
   chat of two, so adding and removing members and devices changes the
   group's keys. The MLS library runs in the client (chosen in slice D0;
   if no Go library fits WebAssembly, a TACHYON request follows).
4. **Transport is WebRTC data channels** (TACHYON request P1), started
   through signalling messages on `realtime`'s socket. When two devices
   cannot connect directly, a **TURN relay** carries the encrypted
   traffic. Servers see who connects to whom and when (metadata), never
   the content.
5. **The person's own devices sync with each other** the same way, as
   members of every group the person is in.
6. **Encrypted backup.** Each person's devices keep an encrypted backup of
   their chat history and keys on the server (a blob store like
   `attachments`, not readable by the server). It is unlocked with a
   recovery code made when the first device is set up; unlocking with the
   passkey follows when TACHYON supports the WebAuthn PRF extension
   (request P3). A new device, or a person who lost every device, restores
   from it.
7. **What the servers still do:** sign-in (`identity`, Keycloak), the
   people, members and device keys (`directory`), signalling (`realtime`),
   the TURN relay, the encrypted backups, push notifications without
   message text (`notify`: "New message"), email, and files sent in chat
   as encrypted blobs. Whether an encrypted store-and-forward mailbox is
   added for devices that are offline is open (section 8, Q5).

## Limits the owner accepted

- **Delivery needs devices online.** Without the mailbox (Q5), a message
  reaches a person only when one of the sender's devices and one of the
  receiver's devices are online at the same time; until then it waits on
  the sender's devices.
- **Small groups only.** A sender's device sends to every device of every
  member, so a group has a maximum size (Q6). The current channels with
  thousands of members cannot work this way (Q6).
- **A relay with public ports on-premise.** An install runs a TURN relay
  that devices on the internet reach: UDP and TCP port 3478, TLS 5349,
  and a UDP port range for relayed traffic, opened in the firewall. This
  amends ADR 0007, which allowed only the HTTPS and SMTP ports inbound.
- **No server search for chat.** Chat is searched only on the device (the
  offline search of slice O5 covers it). Email search on the server stays.
- **Admin limits.** Removing a member removes them from every group's keys
  when the other members' devices are next online; the removed member
  keeps the chats already on their devices. Admins and operators cannot
  read, keep for legal reasons, or delete chat content. SPEC AC-ID-13's
  account actions stay for the account, not for chat content.
- **Lost history without the backup.** A person who loses every device and
  the recovery code loses their chat history.
- **New members see no older messages** unless a member's device shares
  them (open in D0's design, not by default).

## Requests to TACHYON

- **P1, peer connections.** A `Peers` client for Tac services in every
  service language, behind a new browser grant `peers`: create a
  connection, make and accept an offer and an answer, add ICE candidates
  (the app carries all of these over its own server, TACHYON does no
  signalling), open and close data channels (ordered and reliable, and
  unordered), send and receive bytes with back-pressure, and events for
  the connection's state. STUN and TURN server addresses and the TURN
  credentials come only from the app's settings (no public default
  servers, ADR 0007). In browsers and in the native shells (WKWebView on
  macOS and iOS, WebView2 on Windows, WebKitGTK on Linux, Android
  WebView), with the same API; where a platform cannot, it answers
  `unsupported`.
- **P2, encryption on the device.** A `Crypto` client: make X25519 and
  Ed25519 key pairs; X25519 key agreement; Ed25519 sign and verify;
  AES-256-GCM encrypt and decrypt; HKDF-SHA-256; SHA-256; a slow key
  derivation from a recovery code (Argon2id, or PBKDF2-SHA-256 with a high
  count); secure random bytes. Private keys are created inside the client
  and cannot be exported; they are kept in the sealed tables (T6b) or in
  the OS key store (Keychain, Android Keystore, Windows Hello), and they
  are deleted with the device copy. In browsers and native shells.
- **P3, passkey unlock (later).** The WebAuthn PRF extension on
  `Passkeys.get` (TACHYON T13), so a passkey can unlock the encrypted
  backup without a recovery code.

## Consequences

- New servers: the TURN relay (on-premise, with public ports); new routes
  in `directory` (devices and public keys), `realtime` (signalling) and a
  backup blob store; `notify` sends content-free chat pushes.
- `messaging` keeps email; its chat parts, `channels`, chat in `search`
  and the chat fan-out in `realtime` and `notify` are retired at the
  cut-over (SLICES Wave 9).
- Mesail has no release yet, so there is no user chat data to move; test
  data on the dev host is dropped at the cut-over.
- The running chat slices (K5, K9, E3, E8) finish as planned: the server
  chat path keeps working until the cut-over, and `messaging` still
  carries email. Wave 8's chat page app (W2) is where the new chat client
  lives.

## 8. Open questions for the owner

- Q5: an encrypted mailbox for offline devices, or not.
- Q6: the largest group, and what happens to big channels.
- Q7: whether the first release waits for device-to-device chat.
