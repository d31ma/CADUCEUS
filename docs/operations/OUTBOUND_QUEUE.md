# Outbound Queue Operations

CADUCEUS sends hosted-domain mail directly to recipient MX servers. Every
accepted submission enters the durable FYLO `outbox`; the supervised worker
runs one signed, non-overlapping delivery pass every 30 seconds by default.
Do not edit or delete FYLO queue documents manually.

## Detect

Collect the secret-free JSON records prefixed with `[mail:status]`. The
`outboundQueue` object reports worker runs, failures, duration, the last
delivery counters, and:

- `queue.depth`, `queued`, and `sending`;
- `queue.due` and `oldestAgeMs`;
- aggregate `queue.attempts`.

Investigate when the worker is failed, `due` remains non-zero across several
worker intervals, depth grows monotonically for 15 minutes, or oldest age
exceeds the expected retry window. Confirm disk capacity, FYLO readiness, DNS,
outbound TCP port 25, local clock, and recent configuration changes before
forcing a pass.

## Delivery Semantics

- A recipient accepted by an MX is terminal and is never retried.
- A 4xx response, timeout, or incomplete delivery diagnostic retains only the
  unconfirmed recipients with exponential backoff.
- A 5xx response creates a mailbox-local delivery failure notice and suppresses
  only that permanently rejected address.
- Mixed results may bounce permanent recipients while retaining transient
  recipients in the same durable queue item.
- A stale `sending` lease is reclaimed after five minutes following a crash.
- Null MX is a permanent refusal; transient DNS failures never fall back to an
  unverified implicit host.

## Diagnose

1. Check `/readyz` and the latest `[mail:status]` record.
2. Verify the worker is enabled and bounded:
   `OUTBOUND_QUEUE_WORKER=on`, `OUTBOUND_QUEUE_INTERVAL_MS`, and
   `OUTBOUND_QUEUE_TIMEOUT_MS`.
3. Resolve the recipient domain's MX and test outbound TCP 25 from the same
   network namespace as CADUCEUS.
4. Check DKIM key readability and `MAIL_HOSTNAME`/PTR alignment.
5. Inspect only bounded SMTP error summaries. Never copy message bodies,
   authorization headers, DKIM keys, or `INBOUND_WEBHOOK_SECRET` into tickets
   or logs.

## Recover

The worker normally recovers by itself. To request one immediate pass, send an
empty JSON body to the local `/send/flush` endpoint with an HMAC-SHA256 of the
exact body. A normal user or administrator session cannot invoke this system
operation.

```sh
CADUCEUS_API_URL=http://127.0.0.1:8080 node --input-type=module -e '
  import { createHmac } from "node:crypto";
  const body = "{}";
  const signature = createHmac("sha256", process.env.INBOUND_WEBHOOK_SECRET).update(body).digest("hex");
  const response = await fetch(`${process.env.CADUCEUS_API_URL}/send/flush`, {
    method: "POST",
    headers: { "Content-Type": "application/json", "X-Caduceus-Signature": signature },
    body,
  });
  console.log(response.status, await response.json());
'
```

The response includes `before`, pass counters, and final `queue` health. Do not
repeat the command while `sending` is non-zero; processing is serialized, but
repeated manual intervention can obscure the underlying outage.

## Verify

Queue depth should fall, `due` should return to zero, worker failures should
stop increasing, and intended recipients should receive no duplicates. Run
`bun run verify:mail-transport` after transport changes; it performs real
ESMTP/STARTTLS/MIME interoperability and an accelerated, deterministic 72-hour
state-machine soak with partial, transient, and permanent recipient outcomes.

