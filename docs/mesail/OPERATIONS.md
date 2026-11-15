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
| `calls` **(planned deployment, with OP2's successor)** | The forwarding server for group calls (ADR 0012, `edge/calls`). It forwards end-to-end encrypted audio and video, holds no keys and keeps no data. |
| SMTP edge **(planned deployment)** | Receives mail on port 25 and hands it to `mail-in`. |
| TURN relay **(planned, G3)** | coturn, pinned by digest: relays the encrypted traffic of devices that cannot connect directly (ADR 0011). It cannot read what it relays. |
| Keycloak | Passwords, 2-step codes and sessions (one realm, `mesail`), on SQLite. |
| Egress container **(planned)** | The only way out of the install (section 4). |
| Registry **(planned)** | Holds Floci and every Mesail image, so nothing is downloaded at start. |

Nothing else should run on that VM's Docker engine: Floci controls the
engine through its socket, which is the same as root on that engine.

### Scheduled work

Each app runs its own scheduled workers (`@Schedule` in
`server/workers/`). TACHYON runs a worker `initialDelaySeconds` after the
app starts, then again `fixedDelaySeconds` after each run ends. Apps that
start together used to sweep in the same second, and edits stalled at that
second (slice K16). So each worker has its own start time:

- each 60-second worker starts in its own 5-second window of the minute;
- each 60-second worker starts at least 3 seconds away from every 15- and
  30-second worker. Fifteen starts a minute do not fit in twelve 5-second
  windows, so three 60-second workers share a window with a shorter one.

| Worker | Runs every | First run after the start | Second of the minute |
| --- | --- | --- | --- |
| `realtime` `sweep` | 15 s | 17 s | 02, 17, 32, 47 |
| `devicebox` `outbox-relay` | 60 s | 67 s | 07 |
| `mail-out` `deliver` (jobs, then the outbox relay) | 30 s | 10 s | 10, 40 |
| `messaging` `changes-cleanup` | 1 day | 613 s | 13 |
| `channels` `outbox-relay` | 60 s | 82 s | 22 |
| `notify` `notify-jobs` (jobs, then the outbox relay) | 30 s | 25 s | 25, 55 |
| `directory` `directory-removals` | 60 s | 88 s | 28 |
| `messaging` `outbox-relay` | 60 s | 37 s | 37 |
| `directory` `outbox-relay` | 60 s | 43 s | 43 |
| `mail-in` `outbox-relay` | 60 s | 52 s | 52 |
| `devicebox` `devicebox-expiry` | 60 s | 58 s | 58 |

"Second of the minute" counts from the moment the apps start. TACHYON
waits after a run ends, so each run's length moves the later runs: the
times hold best right after a start and slowly move apart after that.
TACHYON's `@Schedule` has no random offset (jitter); fixed times are
enough. A new worker takes a free second and is added to this table.

**Time budget.** A scheduled outbox sweep stops starting new documents
after a short time and reads at most one page of waiting documents:

| App | Time | Page |
| --- | --- | --- |
| `channels`, `devicebox`, `messaging` | 2 s | 25, 25, 20 documents |
| `mail-in`, `mail-out` | 2 s | 100 documents |
| `notify` | 2 s | 25 documents |
| `directory` | 1 s | 3 documents |

A sweep that stops early answers `"more": true`, and the next run goes
on. So a sweep never holds an app's single FYLO writer for long. Events
still go out right after each write (the relay's drain); the sweep only
retries what the drain missed.

**On Floci** the scheduler calls every route in `deploy/floci/stack.json`
`schedules` once a minute (`scheduleSeconds`), one after another. A
one-minute schedule (EventBridge `rate(1 minute)` too) has no seconds
offset, so there the time budget is the protection, not the start times.

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
| One public UDP port to the `calls` container (default 8121), and the address devices reach it on | Group calls of 3 or more people (ADR 0012) | Set the address in `MESAIL_CALLS_PUBLIC_IPS`. Devices that cannot reach the port use the TURN relay (ADR 0011). |
| Public ports to the TURN relay: UDP and TCP 3478, TCP 5349 (TLS), and a UDP relay range (default 49160-49200), and the relay's public address | Chat and two-person calls between devices that cannot connect directly (ADR 0011) | Section 3, "TURN relay". Plan about 1 port of the range per relayed connection. |

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
| `dnsresolver` in `apps/directory` | The resolver for domain checks. Empty (the default) means the host's own resolver (`/etc/resolv.conf` of the host or container). Set a DNS-over-HTTPS (JSON) address only if you run one; its host must then be in `outgoing.hosts`. There is no public default (OP1). |

**Files (attachments and mail)**

All files (attachments, raw received mail, outbound mail) live in **one
blob folder** on the VM's data disk, shared by `attachments`, `mail-in`,
`mail-out`, the router and the SMTP edge (ADR 0007 section 7). Back up
that folder with the data.

| Setting | Meaning |
| --- | --- |
| `YON_ATTACHMENTS_ADAPTER` | `local`, the only adapter: files in the blob folder. |
| `YON_ATTACHMENTS_FOLDER` | The folder for files (local adapter). |
| `YON_ATTACHMENTS_LINK_BASE`, `YON_ATTACHMENTS_LINK_SECRET` | The `/blobs` address and the link signing secret (32 characters or more); the router's `MESAIL_ROUTER_BLOBS_*` use the same values. |
| Removed S3 settings | The S3 blob adapter was removed (ADR 0007, slice OP6). A setting that still names the adapter `s3` or a `bucket` for `attachments` or `mail-in` makes every file call fail (`attachments` answers `503` `[MS3513]`, `mail-in` `503` `[MS2514]`); the service log names the setting. An environment variable whose name starts with `MESAIL_RAW_STORE_S3` stops the SMTP edge at startup. |

**The blob folder and the router on Floci** (slice OP7). A Lambda gets one
file system, so `attachments`, `mail-in` and `mail-out` share one access
point (`"fileSystemShare": "mail"`): each keeps its own FYLO root
(`/mnt/data/<service>`) and all use `/mnt/data/blobs`. Floci keeps each
access point in one Docker volume, named `...efs-<access point id>-<hash>`.
The router mounts only the blob folder of that volume, never the FYLO roots:

1. Find the access point id with Floci's EFS API: `describe-access-points`,
   the access point whose client token is `<creationToken>-mail`.
2. Find the volume: `docker volume ls --filter name=efs-<access point id>-`.
   Exactly one must match; none or several is an error. Floci makes the
   volume only when a function that uses it first starts, so on a fresh
   engine start one such function first (the Floci scripts wake
   `attachments` once and print a warning).
3. Make the folder `blobs` in that volume, owned by uid 10001 (a one-shot
   container that mounts the volume).
4. Mount it into the router with compose's long volume syntax and
   `subpath: blobs` (Docker Engine 26 or newer), read-write because the
   router takes the signed uploads (`PUT`), and set
   `MESAIL_ROUTER_BLOBS_FOLDER` to that mount.

The router and the functions both run as uid 10001. The Floci form of
these steps is `scripts/floci/blobs.sh` with `deploy/floci/compose.blobs.yaml`.

**Browser push** (off by default, SPEC Q19)

| Setting | Meaning |
| --- | --- |
| `notify.push.enabled` (`YON_NOTIFY_PUSH_ENABLED`) | `false` by default. Turn it on only when the install may reach the browser makers' push services (section 4). While it is off, `GET /notify/push-key` answers `503`, the web app hides push (slice N2), and `notify` sends no Web Push. |
| `YON_NOTIFY_VAPID_PUBLIC_KEY`, `YON_NOTIFY_VAPID_PRIVATE_KEY`, `YON_NOTIFY_VAPID_SUBJECT` | The VAPID keys and contact, from secret files. |
| `notify.push.hosts` | The push service hosts allowed (Google FCM, Mozilla, Apple, Microsoft). |

**Calls (`edge/calls`, planned deployment)**

| Setting | Meaning |
| --- | --- |
| `MESAIL_CALLS_KEYS_URL` | Required. realtime's call keys inside the install, for example `http://realtime:8106/api/v1/realtime/calls/keys`. |
| `MESAIL_CALLS_LISTEN` | The HTTP address of the calls routes (the image sets `0.0.0.0:8120`); the router sends `/api/v1/calls/` here. |
| `MESAIL_CALLS_MEDIA_LISTEN` | The one UDP address for all call media (the image sets `0.0.0.0:8121`). |
| `MESAIL_CALLS_PUBLIC_IPS` | The public IP addresses of that UDP port, comma separated. Without it devices are told the container's own address. |

**Signalling and calls on `realtime`** (G2, G11)

| Setting | Meaning |
| --- | --- |
| `YON_REALTIME_CALLS_KEY` | The P-256 key that signs group call join tokens (base64 of its PKCS #8 DER, one line, from a secret file; make one like `identity`'s signing key). Without it `GET /realtime/calls/keys` and group calls answer `503` (MS5042); two-person calls still work. |
| `YON_REALTIME_CALLS_PREVIOUS` | Only during a rotation: the key that signed before. Keep it at least 70 seconds after the switch (join tokens live 60 seconds, `calls` caches the keys 10 minutes but fetches again for an unknown key). |
| `realtime.directoryurl` (`YON_REALTIME_DIRECTORYURL`) | `directory` inside the install (default `http://127.0.0.1:8102`); `realtime` asks it who owns a device. |
| `realtime.notifyurl` (`YON_REALTIME_NOTIFYURL`) | `notify` inside the install (default `http://127.0.0.1:8107`); `realtime` asks it for the "Incoming call" and "Missed call" pushes. |

**TURN relay (planned, G3)**

| Setting | Meaning |
| --- | --- |
| `MESAIL_TURN_URLS` | The addresses devices use, comma separated, for example `turn:turn.example.org:3478?transport=udp,turn:turn.example.org:3478?transport=tcp,turns:turn.example.org:5349`. `realtime` hands them out with the credentials. |
| `MESAIL_TURN_SECRET` | The shared secret of coturn's `use-auth-secret` (from a secret file, at least 32 characters); `realtime` (`YON_REALTIME_TURN_SECRET`) and the relay get the same value. |
| `MESAIL_TURN_PUBLIC_IP`, `MESAIL_TURN_RELAY_PORTS` | The relay's public address (coturn `external-ip`) and its UDP relay range (default `49160-49200`). |
| `realtime.turn.seconds` | How long credentials last (default 43,200 seconds). |
| `MESAIL_TURN_ALLOWED_PEERS` | Empty by default. The relay refuses to relay to private and local addresses (10/8, 172.16/12, 192.168/16, 127/8, 169.254/16, ::1, fc00::/7, fe80::/10) so nobody can use it to reach the install's own network; list address ranges here only to allow one on purpose. |

The relay also runs with `no-cli`, `no-multicast-peers`, a quota of 10
allocations per user, the TLS certificate files of section 2, and its
log on standard output (it never sees message content).

## 4. Traffic that leaves the install

Floci, the functions, Keycloak and `realtime` are on a network with no
route out. Only the egress container **(planned, OP2)** connects out, and
only to:

| Destination | For | When |
| --- | --- | --- |
| Your DNS resolver | Domain checks, SPF, DKIM, DMARC, MX | Always |
| Your mail relay, or any mail server on port 25 | Sending mail outside | Relay, or `mailout.directenabled` |
| The push services in `notify.push.hosts` | Browser push | Only when `notify.push.enabled` is `true` |

Inbound: the router's HTTPS port, the SMTP edge's port 25 and the calls
server's UDP media port **(planned)**. `calls` makes no outbound calls; it
reads only realtime's keys inside the install. Floci's API
port is published on no interface; anyone who reaches it controls every
function and data volume.

Everything else is refused and logged. The OP3 check runs the whole stack
this way and fails on any other outbound attempt, Floci's own included.

## 5. Secrets

Secrets are files on the host, mounted into the VM and passed to the
functions; nothing reads a cloud secret store. Keep them readable only by
the install, back them up separately from data, and never put them in a
log. The full list (client secrets, the link secret, the realtime gateway
secret, `identity`'s signing key, `realtime`'s call key, DKIM keys under `MESAIL_SECRET_*` (one
per hosted domain; `scripts/onprem/dkim-key.sh <domain>` **(planned, OP2)**
makes the key and prints the DNS record), VAPID
keys, the relay login) is in `deploy/floci/README.md` "Keycloak, secrets
and test users" and `stack.json` `secrets`.

Rotation: client secrets, `identity`'s signing key and `realtime`'s call key take a current and a
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
- One live process per FYLO root (ADR 0004 section 4, "One live process
  per root"): a deploy stops a function's old container before its new
  one takes calls, and never runs two containers of one FYLO-owning
  function at the same time. A second container would get `EROOTLOCKED`
  on every store call ("store is not available").

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
