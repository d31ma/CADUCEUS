# ADR 0011: Chat between members goes device to device

- Status: accepted (2026-11-12); the owner answered Q5 to Q7 before
  2026-11-14 (section 8) and suggested QR codes for linking devices
  (section 9)
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
   as encrypted blobs, and the encrypted mailbox for devices that are
   offline (decision 8).
8. **Encrypted mailbox** (the owner's answer to Q5). When a member's
   devices are offline, the sender's device leaves the encrypted message
   in a mailbox per receiving device, on a new small Yon app, `devicebox`
   (a FYLO owner). The server cannot read it. A device fetches its box
   when it comes online and the box deletes what was fetched. A message
   waits at most 30 days; after that it is dropped and the sender's
   devices show "Not delivered". Group changes (adding and removing
   members and devices) travel the same way, so they complete even when
   nobody else is online.
9. **Groups of up to 50 members** (the owner's answer to Q6). A chat group
   (today's channel or a direct conversation) has at most 50 members.
   Public and private groups, joining and leaving stay; a join completes
   when a member's device adds the joiner to the group's keys (through the
   mailbox when nobody is online). The big channels of SPEC AC-CH (up to
   thousands of members) are removed.
10. **The first release waits for this** (the owner's answer to Q7). The
   server chat path never ships: Wave 9 is part of the first release, and
   G8's cut-over happens before it.

## Limits the owner accepted

- **Messages may wait up to 30 days** in the encrypted mailbox, then are
  dropped with "Not delivered" on the sender's devices.
- **Groups of up to 50 members.** A sender's device sends to every device
  of every member; there are no big channels.
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

## Linking a new device: QR code first (the owner's suggestion)

The QR code links a new device to the person's existing one (pairing); the
history itself then moves over the encrypted device-to-device channel.

1. The person signs in on the new device (full sign-in: password and
   2-step code, or a passkey). Signing in alone gives no chat history.
2. On a device they already use, "Add a device" shows a QR code with a
   pairing id, a one-time 32-byte secret, the old device's public key
   fingerprint and an expiry of 2 minutes. The new device scans it. It
   works in either direction: a desktop browser without a camera shows the
   QR code, and the phone scans it.
3. The two devices meet through signalling (G2) by the pairing id,
   connect with P1 (or through the TURN relay), and run a key exchange
   bound to the secret, so nobody else can join. The old device asks
   "Add <device name>?" and the person confirms.
4. The old device signs the new device's public keys and lists them in
   `directory` (G1 accepts a new device key only with a signature by one
   of the person's existing devices, or after a backup restore, step 6),
   and adds the new device to the person's groups.
5. History: if an encrypted backup exists, the new device restores it
   first, then gets only the newer messages from the old device, in parts
   that resume after a break.
6. Fallbacks:
   - a typed code: about 10 characters shown on one device and typed on
     the other, used with a password-authenticated key exchange (CPace),
     so a short code is safe over the relay (G0 picks the library);
   - no old device left: restore from the encrypted backup with the
     recovery code (with the passkey once P3 exists); the backup's signing
     key then signs the new device's keys.

Approving on another device without a code was considered: it needs the
same confirmation and gives weaker proof that the right device is linked,
so QR stays first. No owner choice remains.

Drawing the QR code needs nothing from TACHYON: Mesail's Go client makes
it as an SVG image shown through a `data:` address, which TACHYON's
content security policy allows (`img-src 'self' data:`). Scanning needs a
camera (request P4).

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
- **P4, scanning a QR code.** A `CodeScanner` client for Tac services in
  every service language, behind a new browser grant `camera`, with a
  reason in `tac.application.permissions` like `geolocation`:
  `supported()`, `scan()` and `cancel()`. `scan()` must run from a click
  or key handler; it shows a camera view that TACHYON draws, finds one QR
  code, closes the camera and answers `CodeScanned { text }` or
  `ScanFailed { reason }` (`cancelled`, `denied`, `unsupported`,
  `failed`). The app never receives camera frames. Browsers use the camera
  with the BarcodeDetector API where it exists and a built-in decoder
  elsewhere; the native shells use the platform's scanner (VisionKit or
  AVFoundation on Apple, CameraX with a QR decoder on Android, the Windows
  camera APIs on Windows), with the camera permission text from the
  manifest. Where no camera exists, `supported()` is false.
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
- The server chat path keeps working until the cut-over and never ships.
  K9 (posts in channels of 10,000 members) is dropped. K5, E3 and E8
  finish (they are running or still needed: `messaging` carries email).
  Wave 4's chat parts (the chat sync log and queue) are replaced by G7;
  its email and device-store parts stay. Wave 8's chat page app (W2) is
  where the new chat client lives.

## 8. The owner's answers (before 2026-11-14)

- Q5 "When all of a person's devices are off, where should a new chat
  message wait?": "Server, encrypted (Recommended)" (decision 8).
- Q6 "What is the largest chat group, and what happens to today's big
  channels?": "Up to 50, no big channels (Recommended)" (decision 9).
- Q7 "Should the first release wait for device-to-device chat?": "Yes,
  wait (Recommended)" (decision 10).

## 9. The owner's suggestion

"we can use QR code for syncing between devices/browsers" (2026-11-12):
taken as the first way to link a new device ("Linking a new device").
