# Running Mesail on your own servers

This guide is for the operator of an on-premise Mesail install. It says
what you must provide, which settings you set, and which network traffic
leaves the install. The decisions behind it are in
[ADR 0007](../adr/0007-on-premise-first.md); the requirements are SPEC
AC-OP-4 to AC-OP-8.

Status (2026-11-11): this guide describes the target of slices OP1 to OP3
(SLICES.md "Wave 5"). Items marked **(planned)** do not exist yet. The
integration form that runs today is described in
[`deploy/floci/README.md`](../../deploy/floci/README.md).

## 1. What runs

Mesail runs on one host of yours (kestrel, deployed through PaaS like
FitNest), inside a dedicated virtual machine with its own Docker engine.
In that VM:

| Part | What it is |
| --- | --- |
| Floci | A local emulator of the AWS Lambda, S3 and EFS APIs. It runs every Mesail service as a Lambda container image. It is a production part of the install (SPEC Q18). |
| Lambda functions | One function per service, and per hosted domain for the services that own data (ADR 0004 section 4). |
| Router | The one public web address: the API, the web apps, Keycloak under `/idp`, and the signed file links under `/blobs`. |
| Scheduler | Calls the relay and job routes on a timer. |
| `realtime` | The realtime service and its connection gateway (WebSockets and event streams). |
| SMTP edge **(planned deployment)** | Receives mail on port 25 and hands it to `mail-in`. |
| Keycloak | Passwords, 2-step codes and sessions (one realm, `mesail`), on SQLite. |
| Egress container **(planned)** | The only way out of the install (section 4). |
| Registry **(planned)** | Holds Floci and every Mesail image, so nothing is downloaded at start. |

Nothing else should run on that VM's Docker engine: Floci controls the
engine through its socket, which is the same as root on that engine.

## 2. What you provide

| You provide | Why | Notes |
| --- | --- | --- |
| A VM on your host with its own Docker engine, and a data disk | Everything runs there | Mesail's data (FYLO roots, files, Keycloak) lives on the data disk. Follow ADR 0004's rules for FYLO roots. |
| A public host name and its TLS certificate files | The web address people open | Any CA works, including your own internal CA when only your own devices connect. Mesail has no ACME client. |
| MX records for every hosted domain, and inbound TCP port 25 to the SMTP edge | Receiving mail from outside | Plus SPF, DKIM and DMARC records; the admin app shows the values to publish. |
| A mail relay (smarthost), or outbound port 25 with reverse DNS for your address | Sending mail to outside addresses | Choose one in section 3. |
| A DNS resolver that resolves internet names | Domain checks, SPF, DKIM, DMARC and MX lookups | Use your own resolver. Mesail has no public DNS default (OP1). |
| An NTP server for the host and the VM | Tokens live 60 seconds; 2-step codes, DKIM and file links depend on the clock | Mesail makes no time requests of its own. |
| Secret files, readable only by the install | Client secrets, signing keys, DKIM keys, VAPID keys | Section 5. |
| Backups | FYLO roots, files, Keycloak's database, secrets | Section 7. |

## 3. Settings you set

Names are environment variables or `application.json` keys of the apps.
**(planned)** marks names that slice OP1 or OP2 adds.

**Public address and sign-in**

| Setting | Meaning |
| --- | --- |
| `MESAIL_PUBLIC_URL` | The address people open, for example `https://mail.example.com`. |
| `MESAIL_ISSUER_ORIGIN` | The origin of the token issuer, usually the same as the public address. |
| `MESAIL_ROUTER_RATE_LIMIT` | Sign-in attempts per minute per client address (default 10). |

**Mail out** (`apps/mail-out`)

| Setting | Meaning |
| --- | --- |
| `mailout.relayaddress` | Your relay, as `host:port`. Empty means no relay. |
| `YON_MAILOUT_RELAYUSERNAME`, `YON_MAILOUT_RELAYPASSWORD` | The relay's login, from secret files. |
| `mailout.relayrequiretls` | `true` (default): refuse a relay without TLS. |
| `mailout.directenabled` | `true` to deliver straight to recipients' mail servers on port 25 instead of a relay. Needs outbound port 25 and reverse DNS. |
| `mailout.heloname` | The name the install uses in SMTP; it should match your reverse DNS. |

With neither a relay nor direct delivery, mail to outside addresses is
queued and never sent. Mail between your own hosted domains needs neither.

**Mail in** (SMTP edge)

| Setting | Meaning |
| --- | --- |
| `SMTP_INBOUND_PORT` | Listen port (25). |
| `MAIL_HOSTNAME` | The name in the SMTP banner. |
| `MAIL_TLS_CERT_FILE`, `MAIL_TLS_KEY_FILE` | Certificate files for STARTTLS; a renewed file is read again without a restart. |
| `MESAIL_RAW_STORE_DIR` | Folder for the raw copies of received mail (the local store). |

**DNS**

| Setting | Meaning |
| --- | --- |
| `dnsresolver` in `apps/directory` | The resolver for domain checks. Default after OP1: the host's own resolver **(planned)**; today it is a public DNS-over-HTTPS address and must be changed. |

**Files (attachments)**

| Setting | Meaning |
| --- | --- |
| `YON_ATTACHMENTS_ADAPTER` | `local` (default): files on the data disk. `s3` only with an S3-compatible store of yours. |
| `YON_ATTACHMENTS_FOLDER` | The folder for files (local adapter). |
| `YON_ATTACHMENTS_LINK_BASE`, `YON_ATTACHMENTS_LINK_SECRET` | The `/blobs` address and the link signing secret (32 characters or more); the router's `MESAIL_ROUTER_BLOBS_*` use the same values. |
| An S3 endpoint | Required with the `s3` adapter; after OP1 it never falls back to AWS **(planned)**. |

**Browser push** (off by default, SPEC Q19)

| Setting | Meaning |
| --- | --- |
| `notify.push.enabled` **(planned)** | `false` by default. Turn it on only when the install may reach the browser makers' push services (section 4). |
| `YON_NOTIFY_VAPID_PUBLIC_KEY`, `YON_NOTIFY_VAPID_PRIVATE_KEY`, `YON_NOTIFY_VAPID_SUBJECT` | The VAPID keys and contact, from secret files. |
| `notify.push.hosts` | The push service hosts allowed (Google FCM, Mozilla, Apple, Microsoft). |

## 4. Traffic that leaves the install

Floci, the functions, Keycloak and `realtime` are on a network with no
route out. Only the egress container **(planned, OP2)** connects out, and
only to:

| Destination | For | When |
| --- | --- | --- |
| Your DNS resolver | Domain checks, SPF, DKIM, DMARC, MX | Always |
| Your mail relay, or any mail server on port 25 | Sending mail outside | Relay, or `mailout.directenabled` |
| The push services in `notify.push.hosts` | Browser push | Only when `notify.push.enabled` is `true` |

Inbound: the router's HTTPS port and the SMTP edge's port 25. Floci's API
port is published on no interface; anyone who reaches it controls every
function and data volume.

Everything else is refused and logged. The OP3 check runs the whole stack
this way and fails on any other outbound attempt, Floci's own included.

## 5. Secrets

Secrets are files on the host, mounted into the VM and passed to the
functions; nothing reads a cloud secret store. Keep them readable only by
the install, back them up separately from data, and never put them in a
log. The full list (client secrets, the link secret, the realtime gateway
secret, `identity`'s signing key, DKIM keys under `MESAIL_SECRET_*`, VAPID
keys, the relay login) is in `deploy/floci/README.md` "Keycloak, secrets
and test users" and `stack.json` `secrets`.

Rotation: client secrets and `identity`'s signing key take a current and a
previous value (CONTRACTS.md "identity"); set the new value as current and
the old as previous, restart, then remove the previous value.

## 6. Images and upgrades

- Builds may download pinned, checksum-checked parts from the internet
  (SPEC Q20). A running install never downloads anything (AC-OP-6).
- Build the images, push them to the registry inside the install, then
  deploy. The functions' image addresses point to that registry.
- Floci is pinned by digest. Upgrade it only after OP3 passes with the new
  digest.
- Before every deploy, `fylo pack` every FYLO root (ADR 0004 section 4),
  so a deploy can roll back.

## 7. Backups and restore

| What | How |
| --- | --- |
| FYLO roots | `fylo pack` of each root to `.vdbx` files, before every deploy and on a schedule. |
| Files (attachments, raw mail) | Copy the attachments folder and the raw store folder. |
| Keycloak | Back up the SQLite file in `MESAIL_KEYCLOAK_DATA`. Restoring it keeps every person's id; a person re-created by hand in Keycloak is a new, unknown person to Mesail (CONTRACTS.md "identity"). |
| Secrets | Their own backup, kept apart from the data. |

## 8. Checks

- Every service answers `GET /api/v1/<service>/health`.
- The router answers `GET /_router/ready`.
- Run the OP3 check after any change to the stack, the egress allowlist or
  an image digest **(planned)**.
