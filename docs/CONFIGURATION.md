# Configuration Contract

The authoritative machine-readable definition is
[`contracts/config/v1.schema.json`](../contracts/config/v1.schema.json).

## Loading and Validation

The Go runtime will load configuration in this order:

1. compiled safe defaults;
2. one optional configuration file;
3. environment variables;
4. explicitly supplied command flags.

Later sources override earlier sources. Unknown file keys and malformed known
environment variables fail before listeners open. Secrets are never accepted
as command flags because process listings are not a secret boundary.

## Production Requirements

- `INBOUND_WEBHOOK_SECRET`, `FYLO_ROOT`, `FYLO_BINARY`, `SESAME_BINARY`,
  `SESAME_DEPLOYMENT`, `SESAME_ISSUER`, `EXTERNAL_ACCOUNT_CREDENTIAL_KEYS`,
  and `EXTERNAL_ACCOUNT_ACTIVE_KEY` are mandatory.
- Secret values must come from a root-readable file, container secret, or
  environment injected by the deployment system.
- `SMTP_ADAPTER=console`, test routes, and ACME staging are rejected in
  production.
- SMTP submission requires TLS.
- Enabling direct mail requires a hostname and DKIM configuration.
- Enabling ACME requires a contact email and exclusive ownership of its
  challenge port.
- Enabling FYLO POSIX access requires the locked protocol and capability set.
- The identity deployment must exist before the service starts. `caduceus auth
  init` creates it and refuses to touch an existing one: creating it implicitly
  would let a volume whose keys failed to mount mint a new signing key and end
  every session with no explanation.

## Runtime Keys

| Key | Default | Purpose |
| --- | --- | --- |
| `HOST` | `127.0.0.1` | Public HTTP bind host |
| `PORT` | `8080` | Public HTTP port |
| `METRICS_HOST` | `127.0.0.1` | Operations bind host; non-loopback values are rejected |
| `METRICS_PORT` | `9090` | Operations port for `/metrics`, `/healthz`, and `/readyz` |
| `STATIC_DIR` | `build/static` | Manifested Tachyon frontend copied by `bun run bundle:go` |
| `RUNTIME_LOCK` | `runtime.lock.json` | Pinned runtime artifact identities |
| `SHUTDOWN_TIMEOUT` | `10s` | Bound for draining both HTTP listeners |
| `FYLO_ROOT` | none | Authoritative data root; its parent must be writable for the exclusive lease |
| `FYLO_BINARY` | none | Digest-locked FYLO executable |
| `FYLO_SCHEMA` | `db/schemas` | Versioned document schema directory |
| `FYLO_POSIX_ACCESS` | `false` | Apply FYLO uid/gid/mode ownership on supported POSIX filesystems |
| `FYLO_BRIDGE_ADDRESS` | `127.0.0.1:9876` | Authenticated legacy bridge; explicit loopback IP only |
| `FYLO_BRIDGE_SECRET` | none | HMAC key for split-process use; at least 32 characters |
| `FYLO_INCOMING_ROOT` | `<FYLO_ROOT>/.caduceus-incoming` | Only directory accepted for attachment ingestion |
| `FYLO_REQUEST_TIMEOUT` | `10s` | Bound for one machine operation |
| `FYLO_MAX_REQUEST_BYTES` | `1048576` | Maximum request frame |
| `FYLO_MAX_RESPONSE_BYTES` | `8388608` | Maximum response frame |
| `FYLO_RESTART_LIMIT` | `3` | Child restart budget |
| `FYLO_MAX_ATTACHMENT_BYTES` | `52428800` | Maximum individual attachment |
| `EXTERNAL_ACCOUNT_CREDENTIAL_KEYS` | none | Write-only JSON map of positive versions to base64-encoded 32-byte AES keys |
| `EXTERNAL_ACCOUNT_ACTIVE_KEY` | none | Key version used for new provider-credential encryption |
| `EXTERNAL_ACCOUNT_CONNECTION_TIMEOUT` | `15s` | Combined bound for each IMAP or SMTP validation exchange; `1s` through `2m` |
| `EXTERNAL_ACCOUNT_PRIVATE_CIDRS` | none | Comma-separated canonical private/loopback CIDRs explicitly approved for provider connections |
| `MICROSOFT_OAUTH_CLIENT_ID` | none | Optional Microsoft Entra confidential web-app client ID |
| `MICROSOFT_OAUTH_CLIENT_SECRET` | none | Write-only Entra client secret; at least 16 characters |
| `MICROSOFT_OAUTH_TENANT` | `organizations` | Tenant segment used for Microsoft authorization and token exchange |
| `MICROSOFT_OAUTH_REDIRECT_URI` | none | Exact HTTPS Settings callback registered in Entra; explicit loopback HTTP is accepted for development |
| `GOOGLE_OAUTH_CLIENT_ID` | none | Optional Google confidential web-app client ID for Gmail and Workspace |
| `GOOGLE_OAUTH_CLIENT_SECRET` | none | Write-only Google client secret; at least 16 characters |
| `GOOGLE_OAUTH_REDIRECT_URI` | none | Exact HTTPS Settings callback registered in Google Cloud; explicit loopback HTTP is accepted for development |
| `SNOOZE_WORKER` | `on` | Enable automatic return of due snoozed conversations |
| `SNOOZE_INTERVAL_MS` | `30000` | Delay between due-record scans |
| `SNOOZE_TIMEOUT_MS` | `60000` | Bound for one local snooze return pass |
| `REMINDER_WORKER` | `on` | Enable independent no-reply follow-up resurfacing |
| `REMINDER_INTERVAL_MS` | `30000` | Delay between pending follow-up scans |
| `REMINDER_TIMEOUT_MS` | `60000` | Bound for one local follow-up pass |
| `SMTP_ADAPTER` | `direct` | Hosted-domain outbound transport; `console` is development-only and forbidden in production |
| `SMTP_TLS_POLICY` | `opportunistic` | Upgrade direct-to-MX delivery with STARTTLS when advertised, or use `require` for validated mandatory TLS |
| `SMTP_FROM` | `postmaster@<MAIL_HOSTNAME>` | Default envelope sender when a caller does not supply one |
| `OUTBOUND_QUEUE_WORKER` | `on` | Run the supervised durable retry/bounce worker |
| `OUTBOUND_QUEUE_INTERVAL_MS` | `30000` | Delay between signed non-overlapping queue passes |
| `OUTBOUND_QUEUE_TIMEOUT_MS` | `60000` | Bound for one queue pass through the local API |
| `PROVIDER_NOTIFICATION_WORKER` | `on` | Enable durable Web Push delivery for connected-account new mail |
| `PROVIDER_NOTIFICATION_INTERVAL_MS` | `15000` | Delay between provider notification outbox claims |
| `PROVIDER_NOTIFICATION_TIMEOUT_MS` | `60000` | Bound for one signed provider notification pass |
| `SESAME_BINARY` | none | Digest-locked SESAME executable |
| `SESAME_DEPLOYMENT` | none | Identity deployment: signing, secrets, and snapshot keys, the engine's own FYLO root, and the provisioning registry |
| `SESAME_ISSUER` | none | Absolute `https` base URL tokens are issued under |
| `SESAME_REDIRECT_URI` | `<PUBLIC_URL>/auth/callback` | Exact URI registered for the first-party client; no browser is ever sent there |
| `SESAME_SESSION_LIFETIME` | `12h` | Session bound, from `1m` through `720h` |
| `SESAME_REQUEST_TIMEOUT` | `10s` | Bound for one engine operation |
| `CADUCEUS_AUTH_POLICY` | `passkey-first` | `passkey-first` or `password-and-passkey`; both require MFA assurance |
| `FYLO_ATTACHMENT_QUOTA_BYTES` | `10737418240` | Maximum attachment-bucket bytes |

The current Tachyon delivery adapter also accepts `TRACKING_SECRET`, a
dedicated random key of at least 16 characters for encrypted recipient
tracking capabilities, and `CADUCEUS_PUBLIC_URL`, the credential-free public
origin embedded in delivered tracking resources. Production deployments
should use a unique 32-byte-or-larger tracking secret and an HTTPS origin.
When these are omitted, the compatibility adapter falls back to
`INBOUND_WEBHOOK_SECRET` and `SESAME_ISSUER`; the dedicated values avoid key
reuse and make rotation independent. These adapter-only variables will move
into the authoritative v1 schema when the delivery vertical slice moves to Go.

Direct-to-MX delivery defaults to opportunistic STARTTLS because public SMTP
interoperability still requires delivery to peers that do not advertise TLS.
`SMTP_TLS_POLICY=require` fails closed when STARTTLS is absent or the peer
certificate cannot be validated; use it only when every intended destination
is known to meet that contract. DKIM requires `DKIM_DOMAIN`, `DKIM_SELECTOR`,
and either `DKIM_PRIVATE_KEY` or `DKIM_PRIVATE_KEY_FILE`. Queue detection,
diagnosis, signed recovery, and verification are documented in
[`operations/OUTBOUND_QUEUE.md`](./operations/OUTBOUND_QUEUE.md).

Relative paths in a JSON configuration file resolve from that file's
directory. Relative environment values resolve from the process working
directory. The metrics listener is loopback-only in v1.

`doctor`, `serve`, and `migrate` verify the synchronized frontend manifest,
schema directory, writable FYLO root, runtime lock, and FYLO artifact digest.
The production runtime is FYLO 26.33.02 at the immutable commit and per-target
digests in `runtime.lock.json`. Its protocol-v1 identity handshake is verified
before the private bridge opens. Preview and unknown digests are rejected when
`NODE_ENV=production`.

The FYLO root's parent must be writable because the kernel-owned exclusive
lease sentinel is adjacent to the root. The supplied image therefore uses
`FYLO_ROOT=/data/caduceus`, with `/data` as both the writable volume and lease
parent.

Deployments upgrading from `/data/fylo` or `/data/caduceus/fylo` must stop
CADUCEUS, take a verified filesystem snapshot, and relocate the FYLO root's
contents directly into `/data/caduceus` while preserving ownership, modes, and
supported extended attributes before starting the new image. Never leave the
old `fylo/` directory nested beneath the new root. The entrypoint deliberately
does not move a live or existing store automatically.

## Entry Point and Handler Settings

The container entry point is the Go binary: `caduceus entry [serve|admin:create|domain:migrate|help]`
(`internal/runtime/entry`). `serve` reads the process environment and:

1. starts `caduceus fylo-bridge` (the single FYLO writer) after `caduceus auth init`,
   generating `FYLO_BRIDGE_SECRET` when it is unset;
2. creates every FYLO collection (`internal/platform/fylo/collections.go`);
3. runs the configured bootstrap (`CADUCEUS_BOOTSTRAP_DOMAINS`, or
   `CADUCEUS_ADMIN_EMAIL` with `CADUCEUS_ADMIN_DOMAIN`) through `scripts/create-admin.mjs`;
4. writes `.env` with the handler settings as `YON_*` names, because Tachyon
   gives handler processes no application environment; paths ending in
   `_ROOT`, `_FILE`, `_BINARY` or `_EXEC_PATH` are made absolute, and a
   multi-line value (such as a PEM key) is skipped, so set its `*_FILE` form;
5. runs `ty serve --port $PORT` (default `8080`), adding `--host $HOST
   --allow-non-loopback` when `HOST` is not a loopback address;
6. waits for `GET /health`, then supervises the inbound MX
   (`MAIL_SERVER_ENABLED`), submission (`MAIL_SUBMISSION_ENABLED`), ACME
   renewal (`MAIL_ACME_ENABLED`) and the outbound-queue, snooze, reminder and
   provider-notification workers (each `*_WORKER=off` disables one).

The handler settings are listed once, in `internal/runtime/settings`
(`HandlerSettings`); a test fails when the Go server reads a setting that is not
listed. The entry point refuses to overwrite a `.env` it did not write.

| Variable | Description |
|---|---|
| `TACHYON_BIN` | The `ty` to run, default `ty` on `PATH` |
| `CADUCEUS_GO_BINARY` | The binary that serves `fylo-bridge` and `auth init`, default the running executable |
| `CADUCEUS_SCRIPT_RUNTIME` | Runtime for the operator scripts, default `bun` |
| `CADUCEUS_API_START_TIMEOUT_MS` | How long the entry point waits for `GET /health` before it gives up, default `900000` (a cold start compiles every Go handler) |
| `MAIL_STATUS_INTERVAL_MS` | Interval of the `[mail:status]` log record, default `300000`; `0` disables it |

`caduceus inbound-smtp` runs only the inbound MX against `CADUCEUS_API_URL`
(default `http://127.0.0.1:$PORT`). The server refuses request bodies over
1 MiB, so the MX answers a larger message with a permanent `552`.

## Secret Handling

Values marked `writeOnly` in the schema must be redacted from diagnostics,
logs, metrics, crash reports, and support bundles. `doctor` reports only
presence, source, permissions, and validation status.

### External account credential keyring

`EXTERNAL_ACCOUNT_CREDENTIAL_KEYS` is a JSON object whose keys are canonical
positive integer versions and whose values decode to exactly 32 bytes. For
example, `{"1":"<base64-encoded-32-byte-key>"}`. Generate each value from a
cryptographically secure source such as `openssl rand -base64 32`; never use
the placeholder from `.env.example`, and never reuse a SESAME, bridge,
tracking, or webhook secret.

The keyring is part of the durable mail-store backup set even though it must
be stored outside the FYLO root. Losing a version makes every provider
credential sealed with that version unrecoverable. To rotate, add a new
version, change `EXTERNAL_ACCOUNT_ACTIVE_KEY` to it, restart, and retain all
older versions until a future reseal operation confirms that no credential
references them. Removing a version early fails closed; it never falls back to
another key.

Provider DNS is resolved and checked again by the connector, and the approved
address is pinned for the TCP connection. Public global-unicast addresses are
allowed. Private and loopback destinations are blocked unless their complete
address falls inside `EXTERNAL_ACCOUNT_PRIVATE_CIDRS`; link-local, multicast,
and unspecified destinations remain blocked. Keep this list narrow—for
example, the exact subnet of an on-premises mail server, not an entire site or
host network.

### Microsoft 365 OAuth and shared mailboxes

Microsoft connectivity is optional and does not affect hosted CADUCEUS mail.
Create one confidential web application in Microsoft Entra, register the exact
`MICROSOFT_OAUTH_REDIRECT_URI` (normally
`https://mail.example.com/settings`), and configure delegated consent for
`offline_access`, `https://outlook.office.com/IMAP.AccessAsUser.All`,
`https://outlook.office.com/SMTP.Send`, `Calendars.ReadWrite`, and
`Calendars.ReadWrite.Shared`, `Contacts.ReadWrite`, and
`Contacts.ReadWrite.Shared`. The runtime supplies PKCE and state and requests
those scopes itself; do not place tokens in configuration. Accounts connected
before calendar and contact consent was added must be reconnected once so the
operator can grant the new delegated permissions.

`MICROSOFT_OAUTH_CLIENT_ID`, `MICROSOFT_OAUTH_CLIENT_SECRET`, and
`MICROSOFT_OAUTH_REDIRECT_URI` are atomic. If any one is supplied without the
others, startup fails. The client secret belongs in the deployment secret
manager and is redacted from diagnostics. A shared mailbox additionally needs
the signed-in delegate to hold Microsoft Full Access and Send As rights, and
SMTP AUTH must be allowed by the tenant and mailbox policies. CADUCEUS validates
IMAP selection and SMTP authentication before committing the encrypted grant.
It never sends a test message.

### Gmail and Google Workspace OAuth

Google connectivity is also optional. Create an OAuth 2.0 Web application in
Google Cloud and register the exact `GOOGLE_OAUTH_REDIRECT_URI`. CADUCEUS uses
S256 PKCE, exact state, offline access, and the
`https://mail.google.com/` scope required by Google's IMAP and SMTP XOAUTH2
contract. It also requests `https://www.googleapis.com/auth/calendar.events`
and the read-only
`https://www.googleapis.com/auth/calendar.calendarlist.readonly` collection
scope, plus `https://www.googleapis.com/auth/contacts` for Google People
synchronization and writes. These restricted/sensitive scopes can require Google application
verification for deployments serving users outside the operator's own
Workspace. Accounts connected before calendar and contact consent was added must be
reconnected once. The three Google client values are atomic and the secret must
come from the deployment secret manager. Access and refresh tokens are
exchanged, encrypted, refreshed, and rotated only in the Go owner process; they
never reach browser JavaScript.

### MCP capabilities

MCP does not add a deployment secret or third-party configuration key. Each
mailbox owner explicitly enables it in Settings, chooses the allowed accounts,
folder exclusions, and access level, then rotates a dedicated capability. The
plaintext token is shown once and only its SHA-256 hash is persisted in FYLO.
It is independent of SESAME browser sessions and can be revoked without ending
them.

The MCP endpoint shares the main HTTPS listener at `/mcp`; it is not exposed on
the operations listener. Operators should keep it on loopback or a private
network unless remote MCP access is intentional, terminate TLS before any
untrusted network, preserve the `Authorization` header, enforce the existing
1 MiB request bound, and redact that header from proxy logs. Do not transform a
capability into a query parameter, configuration file checked into source, or
environment variable inherited by unrelated processes.

### The identity deployment

`SESAME_DEPLOYMENT` is not an ordinary directory. It holds:

- `keys/signing.key` — every issued token was verified against it;
- `keys/secrets.key` and `keys/snapshot.key`;
- `fylo-root/` — the engine's security ledger, a separate FYLO root from the
  mail store, because one FYLO process owns one root;
- `caduceus-registry.json` (mode 0600) — the per-tenant OIDC client secrets and
  the identifiers of every role and group this deployment created. SESAME roles
  and groups are immutable, tenant-unique, and cannot be read back by name, so
  losing this file does not lose them; it loses the ability to grant or join
  them again.

Back the whole directory up, mount it durably, and keep it out of the mail
volume's lifecycle. Losing it ends every session and invalidates every issued
token.

## Retired Inputs

`JWT_SECRET`, `AUTH_PASSWORD_PEPPER`, and
`CADUCEUS_AUTH_STEP_UP_MAX_AGE_SECONDS` were removed with the hand-written
authentication they configured. Tokens are signed by the engine's ES256 key,
passwords are Argon2id verifiers it holds, and a credential change requires a
freshly satisfied authentication transaction rather than a timestamp on an
older session. A configuration file still carrying them fails to load.

## Deprecated Legacy Inputs

`FYLO_EXEC_PATH` and inline TLS key material are accepted only by the migration
adapter. The Go service uses `FYLO_BINARY` and file-backed TLS keys. Serverless
and cloud-provider configuration are not part of v1.
