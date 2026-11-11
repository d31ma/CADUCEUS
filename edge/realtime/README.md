# edge/realtime

The realtime connection gateway for Mesail. It is a small long-running Go
program, not a Lambda. It holds the browsers' WebSockets and event streams and
knows no business rules. For every connection it calls the stateless
`realtime` Yon app, and the Yon app pushes events back through the gateway's
management API. It is the local stand-in for AWS API Gateway WebSocket APIs.

## AWS mapping

| Gateway | AWS API Gateway WebSocket |
| --- | --- |
| `POST /api/v1/realtime/gateway/connect` (to the backend) | `$connect` route |
| `POST /api/v1/realtime/gateway/message` (to the backend) | `$default` route |
| `POST /api/v1/realtime/gateway/disconnect` (to the backend) | `$disconnect` route |
| `POST` and `DELETE /@connections/{connectionId}` (management port) | `@connections` management API (`PostToConnection`, `DeleteConnection`) |

## Public routes (`MESAIL_REALTIME_LISTEN`)

- `GET /api/v1/realtime/socket`: WebSocket. The `Origin` header must be in
  `MESAIL_REALTIME_ORIGINS` (a request without `Origin` is allowed; it is not
  a browser). Every client text frame goes to the backend `message` route.
  - Every new connection first gets `{"type":"hello",...}`, so a client
    without an open callback knows to send its token.
  - The first accepted auth must arrive within 10 s. The backend's
    `expiresAt` sets the time when the authentication ends; a newer auth moves
    it. 20 s before that time the gateway sends `{"type":"reauth",...}`.
    At that time the gateway sends
    `{"type":"auth.expired",...,"state":"4401"}` and closes with 4401.
  - Binary frame: close 1003. Frame over 16 KiB: close 1009. 64 events
    waiting for a slow reader: close 1013. Backend `connect` failed: close
    1011. Shutdown: close 1001. Ping every 30 s.
- `GET /api/v1/realtime/stream?ticket=...`: server-sent events. The ticket goes
  to the backend as the first message `{"type":"ticket","ticket":"..."}`.
  Events arrive as `data:` lines; the stream ends with `event: close` and
  `data: {"code":4401}`. A `: keep-alive` comment every 25 s. A missing ticket
  answers 400; a failed `connect` answers 503.
- `GET /api/v1/realtime/gateway-health`: `{"status":"ok"}`.

## Management routes (`MESAIL_REALTIME_MANAGE_LISTEN`)

Every request needs the header `X-Realtime-Gateway` with the shared secret,
else 401.

- `POST /@connections/{connectionId}`: the body is the event JSON (at most
  64 KiB). Answers 200, or 410 when the connection is gone or its queue is
  full.
- `DELETE /@connections/{connectionId}?code=4401`: closes the connection with
  that code (default 1000; only 1000 or 4000-4999). Answers 204, or 410.

## Environment variables

| Name | Default | Meaning |
| --- | --- | --- |
| `MESAIL_REALTIME_LISTEN` | `127.0.0.1:8116` | Public listen address |
| `MESAIL_REALTIME_MANAGE_LISTEN` | `127.0.0.1:8117` | Management listen address |
| `MESAIL_REALTIME_BACKEND` | `http://127.0.0.1:8106` | Base URL of the `realtime` Yon app |
| `MESAIL_REALTIME_GATEWAY_SECRET` | required | Shared secret, at least 32 characters; the same value as `YON_REALTIME_GATEWAY_SECRET` |
| `MESAIL_REALTIME_ORIGINS` | none | Comma list of allowed browser origins |

The program refuses to start without a valid secret.

## Running

```sh
cd edge/realtime
MESAIL_REALTIME_GATEWAY_SECRET=... go run .
go test -race ./...
```

Logs are JSON lines on stderr: the event name, the connection id and the
close code. They never hold the secret, tokens or frame contents. `SIGINT` or
`SIGTERM` closes every connection with 1001, calls `disconnect` for each, and
stops both listeners.

## Network

Run the gateway on a private network. The router sends only
`/api/v1/realtime/socket` and `/api/v1/realtime/stream` to the public port.
Never route the management port, or the backend's
`/api/v1/realtime/gateway/*` routes, publicly: the shared secret is the only
check there.

## Packages

- `startup`: reads the environment.
- `backend`: the HTTP client for the three backend routes.
- `connections`: one connection's lifecycle (`Session`, `Writer`), the
  registry (`Hub`) and the backend calls (`Relay`). No socket code.
- `transport`: the WebSocket, event stream, health and management handlers.
