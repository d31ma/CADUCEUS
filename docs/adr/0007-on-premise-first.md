# ADR 0007: On-premise first: no cloud account, no hidden outbound traffic

- Status: Accepted, with four owner decisions open (SPEC Q17 to Q20); the
  defaults below apply until the owner decides.
- Date: 2026-11-11
- Amends: [ADR 0004](0004-mesail-go-serverless-keycloak.md) section 4
  (packaging, SMTP, static frontends) and section 9 (environments). The
  rest of ADR 0004, and ADRs 0005 and 0006, stay.
- Spec: [SPEC.md](../mesail/SPEC.md) AC-OP-4 to AC-OP-8. Slices:
  [SLICES.md](../mesail/SLICES.md) "Wave 5: on-premise".

## Context

The owner's requirement (2026-11-11): "can we make sure this project is
not dependent on any cloud resource, that it can run independently
on-premise".

An audit of `develop` (2cda510) found:

- **Already cloud-neutral.** No code uses an AWS SDK, Secrets Manager,
  SSM, SQS, SNS, KMS or CloudWatch. The "AWS" parts speak AWS-shaped HTTP
  APIs (Lambda Invoke, S3 GetObject, hand-written SigV4) to a configured
  address, which is the local Floci emulator today. Secrets come from
  files or environment variables. The web apps load nothing from outside
  (CSP `'self'`; no fonts, CDNs or analytics). Blobs use the local
  adapter by default. TLS certificates for SMTP are files. There is no
  EventBridge code: a local scheduler calls the job routes.
- **Run-time traffic that leaves the install:**
  1. `directory`'s domain DNS checks default to Cloudflare's public
     DNS-over-HTTPS (`apps/directory/server/application.json`
     `dnsresolver`). This is a public default, and must go.
  2. The S3 blob adapter falls back to `s3.<region>.amazonaws.com` when no
     endpoint is set (`kit/blobs/S3Settings.go`, and the SMTP edge's raw
     store). This is a hidden cloud default, and must go.
  3. Outbound mail: a relay (smarthost) or direct SMTP to recipients' mail
     servers (`mailout.relayaddress`, `mailout.directenabled`). This is
     email itself, not a cloud service.
  4. SPF, DKIM and DMARC checks and MX lookups use the system resolver.
     This is DNS itself.
  5. Browser push goes through the browser vendors' push services (Google
     FCM, Mozilla, Apple, Microsoft). This is a third-party service.
  6. The Lambda form needs a Lambda API endpoint (Floci, or AWS) for every
     request, and serves the web apps from an S3 bucket.
  7. Clocks must be in sync (60-second tokens, TOTP, DKIM `x=`, link
     expiry). The operating system's time service may use public NTP.
- **Build-time downloads** (all pinned by digest or SHA-256 except one):
  container images (Docker Hub, quay.io, public.ecr.aws), Maven Central
  (`sqlite-jdbc`), Ubuntu apt (not pinned), go.dev and GitHub (Go, `ty`,
  `fylo`, Tailwind, DuVay when re-vendored), and the Go module proxy for
  the realtime gateway's one dependency (not vendored).
- **Docs** assume AWS later in ADR 0004 (SES, S3 plus CloudFront, API
  Gateway WebSocket, EventBridge), SPEC's hosting line, CONTRACTS,
  EVENTS, DOMAIN, SLICES P1 and `deploy/floci/README.md`.

## Decision

1. **On-premise is the primary target.** Every Mesail feature runs on the
   owner's own machines with no cloud provider account, no managed cloud
   service and no SaaS, at run time and at deploy time. A cloud target
   (AWS) stays an option that may be added later; it is never required,
   and no code may assume it.
2. **No hidden outbound traffic.** At run time Mesail opens connections
   only to hosts named in its settings. No setting has a public default:
   - DNS for domain checks, SPF, DKIM, DMARC and MX uses the resolver the
     operator names (`dnsresolver`, default: the host's own resolver);
     never a public resolver by default.
   - Outbound mail goes to the relay the operator names, or directly to
     recipients' mail servers when the operator turns direct delivery on.
   - Blob storage is the local adapter, or an S3-compatible store whose
     endpoint the operator names (for example a self-hosted one). The S3
     adapter refuses to start without an endpoint; no AWS fallback.
   - Browser push follows SPEC Q19.
   - Time: the operator points the hosts at their own NTP server. Mesail
     itself makes no time requests.
3. **Run time never downloads.** Images, binaries, vendored modules and
   web assets are complete when deployed. Builds may download only pinned,
   checksum-checked parts (SPEC Q20 decides whether builds must also work
   with no internet). Every Go build runs with `GOPROXY=off` and vendored
   modules.
4. **The on-premise runtime form** (SPEC Q18; default until the owner
   decides: **plain service processes**). Each Yon service runs as one
   long-running `ty start` process per service for all domains (one writer
   per service root, as native development already runs), behind Mesail's
   router in its native mode, with the local scheduler for job routes, the
   realtime container, the SMTP edge and Keycloak, all as containers on the
   owner's host (kestrel through PaaS). The router serves the web apps from
   files. The Lambda images and the Floci form stay as an optional,
   tested target for a later cloud move, not as the production form.
   - Why: Floci is an emulator for testing, third-party, runs with the
     host's Docker socket and its own outbound traffic is not audited; the
     Lambda form adds a queue per single-writer function (the cause of the
     Floci 502 deadlock); and plain processes are already how every slice
     is tested.
   - The other choice (keep the Lambda images in Floci on kestrel as the
     production form, like FitNest) also needs no cloud account; it is
     the current plan in ADR 0004.
5. **What replaces each cloud-shaped part on-premise** (no AWS):

   | Cloud-shaped part | On-premise |
   | --- | --- |
   | Lambda per service (and per domain) | one `ty start` process per service (Q18 default) |
   | API routing / API Gateway | Mesail's router (native mode) |
   | API Gateway WebSocket | `edge/realtime` and the `realtime` container (already) |
   | EventBridge schedules | the local scheduler (already) |
   | S3 for blobs | the local adapter on the host's disk, or a self-hosted S3-compatible store with an explicit endpoint |
   | S3 plus CloudFront for web apps | the router serves the built files |
   | EFS | the host's local disk for FYLO roots (ATLAS rules stay) |
   | Secrets Manager | secret files on the host, read once at start (as now) |
   | SES (in and out) | `edge/smtp` for inbound; the operator's relay or direct delivery outbound |
   | Public DNS default | the operator's resolver |
   | Public CA (ACME) | certificate files from the operator's CA or any CA; no ACME client in Mesail |
   | Public NTP | the operator's NTP server (host setting) |

6. **A gate proves it.** An on-premise run starts the whole stack with all
   outbound traffic blocked except the configured relay, resolver and (if
   on) push services, and the blackbox tests pass, except the ones that
   need external mail or push (SPEC AC-OP-8).

## Consequences

- No feature waits for a cloud account, and no data leaves the install
  except the email, DNS and push traffic the operator configures.
- Four small code fixes (DNS default, S3 fallback, vendoring one module,
  a push on/off setting) and one deployment slice; most of the work is
  already done in native mode.
- If Q18 keeps plain processes, the "serverless-first" shape of ADR 0004
  becomes an optional later target; the code stays the same, because the
  Lambda form was always "a deployment choice, not a code change"
  (ADR 0004 section 4).
- Email to and from other organizations still needs the internet (SMTP
  and DNS). An air-gapped install (SPEC Q17) has only internal domains.
