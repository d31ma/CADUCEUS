# edge/smtp

The inbound SMTP listener (the MX) for Mesail. It is a long-running Go
program, not a Lambda. It accepts mail from other mail servers for hosted
addresses only, checks SPF, DKIM and DMARC, stores one raw copy per recipient
domain, and hands each copy to `mail-in`.

## What it does

1. `RCPT TO` asks `directory` (`GET /api/v1/directory/addresses/{address}`):
   - 200: the address is hosted, the recipient is accepted.
   - 404 with code `domain_not_hosted`: `550 5.7.1` (no open relay).
   - 404 with code `not_found`: `550 5.1.1` (unknown user).
   - any other answer, including a 404 with another code or no code:
     `451 4.4.3` (temporary; the sender retries).
2. `DATA` reads the message up to the size limit, then:
   - checks SPF, DKIM and DMARC (`kit/mailauth`). A DMARC fail with a
     `reject` policy answers `550 5.7.1` and stores nothing.
   - reads the headers (`kit/mime`).
   - for each recipient domain, writes the raw message with an
     `Authentication-Results` header field in front of it to the blob store
     (`kit/blobs`) under the key `<domainId>/inbound-<sha256 hex>`. The hash
     covers the original message, the envelope sender and that domain's
     recipients, so a retried transaction gets the same key. A key that
     `kit/blobs` refuses is never written.
   - calls `POST /api/v1/mail-in/messages` with that domain's recipients.
   - answers `250 2.0.0` only after every domain got `201`. Any failure
     answers `451 4.3.0`, so the sending server tries again.
3. It logs one line per transaction: ids, reply code, sizes and counts. It
   never logs message content, addresses or tokens.

## Packages

- `protocol`: the SMTP conversation (sockets, STARTTLS, limits).
- `reception`: the delivery pipeline and its ports. It has no socket code.
- `adapters`: HTTP directory, HTTP mail-in, the raw store on `kit/blobs`, service tokens.
- `mailchecks`: adapters for `kit/mailauth` and `kit/mime`.
- `startup`: reads the environment and wires the parts together.

## Managed inbound seam

`reception.Reception.Deliver(ctx, reception.Transaction)` is the whole
delivery pipeline. A managed inbound service (for example an SES receipt rule
that writes to S3 and starts a function) can build a `Transaction` from the
event (envelope sender, recipients with their `domainId`, raw bytes, remote
address) and call the same `Deliver`. Only the SMTP socket code in
`protocol` is then replaced; the auth check, raw copies and mail-in calls
stay the same. `Deliver` returns `reception.Refused` for a DMARC reject and
any other error for a temporary failure.

## Environment variables

| Name | Default | Meaning |
| --- | --- | --- |
| `SMTP_INBOUND_HOST` | `0.0.0.0` | Listen address |
| `SMTP_INBOUND_PORT` | `25` | Listen port |
| `MAIL_HOSTNAME` | machine name | Name in the banner and in `Authentication-Results` |
| `SMTP_MAX_SIZE` | `26214400` | Largest message in bytes (25 MiB) |
| `SMTP_MAX_CONNECTIONS` | `200` | Open connections at once; more get `421 4.3.2` |
| `MAIL_TLS_CERT_FILE`, `MAIL_TLS_KEY_FILE` | none | Set both to offer STARTTLS. The files are read again when they change, so a renewal needs no restart |
| `MAIL_AUTH_VERIFY` | on | `off` skips SPF, DKIM and DMARC |
| `MESAIL_DIRECTORY_URL` | required | Base URL of `directory` |
| `MESAIL_MAIL_IN_URL` | required | Base URL of `mail-in` |
| `MESAIL_RAW_STORE_DIR` | required without `MESAIL_RAW_STORE_S3_BUCKET` | Local folder for raw message copies (local development) |
| `MESAIL_RAW_STORE_S3_BUCKET` | none | Set it to keep raw copies in S3 instead of the local folder |
| `MESAIL_RAW_STORE_S3_REGION`, `MESAIL_RAW_STORE_S3_ACCESS_KEY_ID`, `MESAIL_RAW_STORE_S3_SECRET_ACCESS_KEY_FILE` | required with `MESAIL_RAW_STORE_S3_BUCKET` | S3 region, access key id, and a file that holds the secret access key |
| `MESAIL_RAW_STORE_S3_TIMEOUT_SECONDS` | `300` | Longest time for one S3 upload, in seconds (5 minutes is enough for 25 MiB at about 100 KB/s) |
| `MESAIL_RAW_STORE_S3_ENDPOINT` | required with `MESAIL_RAW_STORE_S3_BUCKET` (no default) | The S3-compatible store's address, for example a self-hosted store, or Floci in local tests. There is no cloud default (ADR 0007): without it the edge refuses to start |
| `MESAIL_SERVICE_TOKEN` | none | A fixed Service bearer token |
| `MESAIL_SERVICE_TOKEN_URL`, `MESAIL_SERVICE_CLIENT_ID`, `MESAIL_SERVICE_CLIENT_SECRET_FILE` | required without `MESAIL_SERVICE_TOKEN` | OAuth2 client credentials; the token is kept until 30 seconds before it expires |

## Raw message store

The edge picks the store when it starts:

- With `MESAIL_RAW_STORE_S3_BUCKET` set, it writes each copy to
  `<bucket>/<domainId>/inbound-<hash>` with a signed S3 `PUT`. The secret
  access key is read from the file in
  `MESAIL_RAW_STORE_S3_SECRET_ACCESS_KEY_FILE`, never from a plain variable.
  A missing or empty file stops the edge at startup. S3 uploads use their own
  HTTP client with a 5-minute timeout (`MESAIL_RAW_STORE_S3_TIMEOUT_SECONDS`),
  not the 30-second client of the directory and mail-in calls.
- Without it, it writes each copy to `MESAIL_RAW_STORE_DIR/<domainId>/inbound-<hash>`
  (written to a temporary file, synced, renamed).

## Limits

- 100 recipients per message (then `452 4.5.3`).
- Command lines up to 1000 bytes (longer lines get `500 5.5.2`).
- 10 errors per connection, then `421` and the connection closes.
- 5 minutes to wait for a command, 10 minutes for the message data.
- `AUTH` is not offered on the MX (`502 5.5.1`).
- `SIGINT` or `SIGTERM` stops accepting, waits up to 30 seconds for open
  sessions, then closes them.
