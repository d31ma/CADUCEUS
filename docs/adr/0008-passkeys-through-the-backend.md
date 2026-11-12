# ADR 0008: Passkeys (Touch ID, Face ID, Windows Hello, Android) through the backend

- Status: Accepted (design). The owner decided SPEC Q21 (passkeys are an
  extra way to sign in; passwords stay) and Q22 (biometric unlock of the
  device copy in the installed apps) on 2026-11-12. Build after TACHYON
  offers passkey access to Tac (requests T13, T14) and, for the unlock,
  T15.
- Date: 2026-11-12
- Amends: [ADR 0004](0004-mesail-go-serverless-keycloak.md) section 7
  ("no Keycloak login page, so no passkeys yet") and
  [ADR 0005](0005-one-login-per-person.md) (one login, now with passkeys).
- Spec: [SPEC.md](../mesail/SPEC.md) S18, AC-ID-15 to AC-ID-19, Q21, Q22.
  Contracts: [CONTRACTS.md](../mesail/CONTRACTS.md) "identity"
  (passkeys). Slices: [SLICES.md](../mesail/SLICES.md) "Wave 6: passkeys".

## Context

The owner's request (2026-11-12): "i would like to add biometrics (e.g.
Touch ID, Face ID etc) as a keypass option for authenication".

A biometric sign-in on the web and in apps is a **passkey**: a WebAuthn
credential kept by the device's platform authenticator (Touch ID, Face
ID, Windows Hello, Android biometrics). The biometric never leaves the
device; the server stores only a public key and checks a signature.

Rules that stay: only the backend talks to Keycloak; the browser holds
only Mesail's own token (ADR 0005); Mesail stores no credential (AC-ID-5);
one login per person; passwords keep working.

What exists:
- Keycloak 26.8.2 supports WebAuthn and passkeys, but only in its own
  browser login pages. Its direct-grant flow, which `identity` uses, has no
  WebAuthn step. `mesail-keycloak` already ships a small provider (the OTP
  admin endpoint), so a Keycloak extension is an established pattern.
- TACHYON (develop, 2026-11-12) has no WebAuthn client for Tac, no native
  passkey bridge (no `ASAuthorization`, no Android Credential Manager), and
  the Apple scheme `tachyon-app://` is not a secure context. A WebView
  can use WebAuthn only for its own origin, which is never the install's
  host, so native apps need a native bridge.

## Options

**(a) A Keycloak extension accepts a WebAuthn assertion that `identity`
passes through.** Chosen. Keycloak keeps the passkey (public key, sign
count) as a credential of the user; a custom direct-grant authenticator
in `mesail-keycloak`'s provider checks an assertion with Keycloak's own
WebAuthn code; `identity` relays options and results between the browser
and Keycloak, as it already does for passwords and TOTP. The browser never
talks to Keycloak, Mesail stores no credential, Keycloak's brute-force
detection and session rules apply, and refresh, domain switch, kept
devices and "sign out everywhere" work unchanged.

**(b) Keycloak's own login page for passkeys.** Rejected: the browser would
talk to Keycloak and get Keycloak's redirects and cookies, against the
owner's rule (ADR 0004 section 7).

**(c) `identity` as the WebAuthn relying party, keeping public keys in
FYLO.** Rejected: Mesail would store credentials (against AC-ID-5), and
after checking a passkey `identity` would need a Keycloak session for a
user without a password, which needs Keycloak's impersonation or token
exchange power: one stolen `identity` secret could then sign in as anyone.

## Decision

1. **Passkeys live in Keycloak**, one realm `mesail` (ADR 0005), as
   WebAuthn passwordless credentials of the person. Relying party id: the
   install's public host name (one per install, for all domains, so one
   passkey serves every domain of the person). User handle: the person id.
   User verification required (the biometric or the device PIN), so a
   passkey sign-in counts as two factors and needs no 2-step code.
   Attestation: none. At most 10 passkeys per person.
2. **Sign-in with a passkey** (no password typed): `identity` makes a
   challenge (random 32 bytes), seals it with its session key, bound to a
   2-minute expiry, and answers WebAuthn request options for a
   discoverable credential, so the person picks a passkey without typing
   a username. The browser (or the native app) asks the device; `identity`
   sends the assertion, the expected challenge and the allowed origins to
   Keycloak's token endpoint through the passkey direct-grant flow
   (`mesail-keycloak` provider). Keycloak finds the user from the user
   handle, checks the signature, sign count and user verification, and
   answers tokens like a password sign-in. Then everything follows ADR
   0005: Mesail token, default domain (or a chosen one), `sessionKey`,
   kept devices (ADR 0006).
3. **Adding a passkey** is an account setting, like the 2-step code: the
   person signs the request with their current password (and code), the
   device creates the passkey, and `identity` gives the attestation to the
   provider's admin endpoint (client `mesail-account`), which stores it for
   the caller's own `sub` only.
4. **Removing a passkey** needs the current password (and code) or another
   passkey; it ends every other session of the person
   (`sessionsEndedAt`, ADR 0006), so a lost device that held a session is
   signed out. An admin account action (SPEC AC-ID-13) can remove all of a
   person's passkeys.
5. **Native apps** use the platform's passkey API through TACHYON
   (request T14): Apple `ASAuthorization` with a `webcredentials`
   associated domain, Android Credential Manager with Digital Asset Links,
   Windows `webauthn.dll`. Each install publishes
   `/.well-known/apple-app-site-association` and
   `/.well-known/assetlinks.json` from its router, and its app builds name
   the install's host. Linux apps have no platform passkey; they keep
   password sign-in. `identity` accepts the web origin and the apps'
   origins listed in its settings.
6. **Recovery** keeps the password: a person who loses a device signs in
   with password and code and removes that passkey; a person who loses
   everything gets an admin reset (AC-ID-13), which can also remove all
   passkeys. Every account keeps its password (the owner's answer to SPEC
   Q21); there are no passkey-only accounts.
7. **Unlocking the offline copy with a biometric** (the owner's answer to
   SPEC Q22: yes, in the installed apps; SPEC AC-OF-20). It needs sealed tables (ADR 0006 section 9) whose
   device key the OS releases only after Touch ID, Face ID, Windows Hello
   or an Android biometric (TACHYON request T15). It is for the installed
   apps only: a browser has no such key store.

## Consequences

- One more sign-in path with no new credential store in Mesail; Keycloak
  stays the single owner of every credential.
- `mesail-keycloak` grows its provider (a passkey authenticator and admin
  endpoints); it must track Keycloak's WebAuthn internals on upgrades.
- Web passkeys wait for TACHYON T13, native ones for T14; until then
  nothing changes for users.
- Each on-premise install builds its own native apps with its host in the
  associated domains, because the apps' passkeys are bound to that host.

## Requests to dependencies (TACHYON)

TACHYON's plan was approved by the owner on 2026-11-12 (TACHYON's
coordinator notes, `tachyon-passkeys-plan.md`). Order: **T13** in TACHYON's
wave 2 (started), **T15** right after the sealed tables (wave 3b), **T14**
first in wave 4. Device evidence for T14 and T15 needs resources the owner
provides; TACHYON's coordinator handles that.

- **T13** (web) A `Passkeys` page client in every Tac service language,
  behind the browser grant **`passkeys`**: `supported()`, `check()`,
  `create(optionsJson)`, `get(optionsJson)`, `offer(optionsJson)`
  (conditional mediation: passkeys in the username field's autofill; web
  only in v1) and `cancel()`. Options are WebAuthn Level 3 JSON (at most
  64 KiB). Success arrives as `PasskeyCreated { credentialJson }` or
  `PasskeyAsserted { credentialJson }`; every failure as
  `PasskeyFailed { call, reason, message }` with `reason` one of
  `cancelled`, `exists`, `unsupported`, `refused`, `busy`, `invalid`,
  `failed`. A user action is required. **On TACHYON `develop`** since
  2026-11-12 (b2ea5b2aa, TACHYON ADR 0126; not released yet): the grant is
  `passkeys` in `tac.browserApis`; `check()` answers with a
  `PasskeySupport` event; `create` and `get` run only from a click or key
  handler; native apps answer `unsupported` until T14.
- **T14** (apps) The same client in the native shells through each
  platform's passkey API (Apple `ASAuthorization`, Android Credential
  Manager, Windows `webauthn.dll`; Linux answers `unsupported`). The
  relying party comes from the manifest member
  `tac.application.passkeys.relyingParty` (the install's host). The origin
  in the credential is `https://<relyingParty>` on Apple and Windows, and
  `android:apk-key-hash:<hash>` on Android (the build prints the hash), so
  `identity.passkeyorigins` lists those for each install (CONTRACTS
  "identity").
- **T15** (apps, firm since SPEC Q22) Sealed tables and blobs with
  `"unlock": "userPresence"`: rows sealed under a second key that the OS
  releases only after a biometric or the device PIN; a `StorageLock`
  client (`state()`, `unlock()`, `lock()`) with built-in events
  (`StorageUnlockAsked`, `StorageUnlocked`, `StorageUnlockRefused { reason
  }` with `cancelled`, `failed`, `lockedOut`, `notEnrolled`, `keyLost`,
  `StorageLocked { reason }`); no reads or writes while locked (error
  `storage_locked`); relock after `tac.storage.unlock.relockAfterSeconds`
  (default 300) in the background and when the device locks; the prompt
  text from `tac.storage.unlock.reason`. Mesail declares
  **`"unlock": "userPresenceWhenAvailable"`**, so the web and Linux builds
  use the normal sealed key and get `StorageUnlockUnavailable` (SPEC
  AC-OF-20 leaves them out).
- **T15b** (accepted by TACHYON on 2026-11-12, with the owner's OK; part of
  the T15 slice in wave 3b; final names come from T15's ADR) A run-time
  switch for the unlock key, because
  SPEC AC-OF-20 makes the biometric lock a choice of the person, while T15
  fixes it per table in the declaration: `StorageLock.require(true|false)`.
  Off: the unlock key is kept under the normal sealed device key (no
  prompt). On: it is moved under the OS-gated key. Changing it needs a
  successful `unlock()` and answers `StorageLockRequired { required }`.
  Where no OS lock exists, `require(true)` follows T15's rule (refused, or
  Mesail's `userPresenceWhenAvailable` fallback with
  `StorageUnlockUnavailable`).
