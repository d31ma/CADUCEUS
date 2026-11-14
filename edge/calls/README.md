# calls: the forwarding server for group calls

`calls` forwards the audio and video of group calls (3 to 50 people,
ADR 0012). It is a plain Go module on Pion, like `edge/realtime`. It is
not a Yon app and not a Lambda: it runs as one long-running container.

It never has the media keys. Every frame is SFrame-encrypted on the
sending device, and `calls` forwards the bytes as they arrive. It keeps
only who is connected to which call, in memory. After a restart the calls
are gone and devices join again.

Contract: `docs/mesail/CONTRACTS.md`, section "calls". Error numbers:
MS6500 to MS6512 in `docs/mesail/ERRORS.md`.

## Routes

| Route | Token (header `X-Mesail-Join`) | Answer |
| --- | --- | --- |
| `POST /api/v1/calls/{callId}/offer` with `CallOffer{sdp}` | unexpired | `200 CallAnswer{sdp}` |
| `POST /api/v1/calls/{callId}/candidates` with `CallCandidates{candidates[]}` (1 to 20 `RTCIceCandidateInit`) | may be expired while the device is in the call | `204` |
| `POST /api/v1/calls/{callId}/leave` | may be expired while the device is in the call | `204` |
| `GET /api/v1/calls/health` | none | `200 Health{service, status}` |

The join token is a compact JWS from `realtime` (slice G11): ES256, `kid`
in the header, claims `callId`, `deviceId`, `domainId`, `iat`, `exp` and
`aud: "calls"`. `calls` reads the public keys from realtime's JWKS
(`MESAIL_CALLS_KEYS_URL`), keeps them 10 minutes, and asks again at most
every 10 seconds when a token names an unknown key. Until G11 lands, the
tests sign tokens with `fakerealtime.JoinKeys`.

The answer holds all of the server's ICE candidates, all on the one media
port. The server does not trickle candidates to the device.

## What a device offers (fixed slots)

The server never sends a new offer. So a device offers every slot it
wants at once, as media lines in this order of kinds:

| Media line | Direction in the device's offer | Count |
| --- | --- | --- |
| Microphone | `sendonly` audio | 0 or 1 |
| Camera, then shared screen | `sendonly` video | 0, 1 or 2 (the first is the camera, the second the screen) |
| Screen slot | the first `recvonly` video | 0 or 1 |
| Video slots | the next `recvonly` video lines | up to 9 |
| Audio slots | `recvonly` audio | up to 49 |
| Data channel | label `calls` | 1 |

Media lines of the same kind and direction are counted in offer order;
audio and video lines may be mixed. A device that does not share a
screen still offers the camera line first. Anything above the counts is
answered `400` MS6509. Codecs: Opus for audio, VP8 for video.

Header extensions the device sends (they are not encrypted):
- `urn:ietf:params:rtp-hdrext:ssrc-audio-level` on audio: who speaks.
- `rtp-stream-id` with simulcast layers `q`, `h` and `f` on the camera.
- Keyframes: the server reads them from the VP8 header, which TACHYON's
  SFrame transform keeps in clear (the payload descriptor and the first
  byte of the frame: start of partition 0 with the inter-frame bit off).
  The `http://tools.ietf.org/html/draft-ietf-avtext-framemarking-07`
  extension's "independent" bit counts too, when a device sends it. A
  layer or person switch happens at a keyframe; when none is seen, the
  server switches at the next frame start 300 ms after it asked for one.

## What the server sends back

- Each video slot shows one person's camera; the people who spoke most
  recently come first, and a person keeps a slot while still among the
  first 9. Each audio slot carries one person's microphone in the same
  order. The screen slot carries the screen shared first.
- Simulcast layer by bandwidth (send-side estimate from transport-wide
  congestion control): the first speaker gets the best layer that fits,
  the others the smallest that fits.
- A keyframe request (PLI or FIR) from a device for a slot goes to the
  person in that slot, at most every 250 ms per stream.
- The data channel `calls` gets this JSON whenever it changes:
  `{"type":"slots","video":[deviceId or ""...],"screen":deviceId or "","audio":[...],"order":[deviceIds, latest speaker first]}`.
  `video[i]` is the person in the i-th video slot. The device picks the
  SFrame key by the key id in each frame, not by the slot.

## Clean-up

A device leaves with `leave`, or when its connection fails (5 seconds
without traffic, then 15 seconds to fail), or when it has not connected
20 seconds after its offer. The call ends when its last device leaves. A
second offer from the same device replaces its first connection. The
51st device is answered `409 call_full` (MS6506).

## Settings

| Variable | Default | Meaning |
| --- | --- | --- |
| `MESAIL_CALLS_KEYS_URL` | required | realtime's call keys, for example `http://realtime:8106/api/v1/realtime/calls/keys` |
| `MESAIL_CALLS_LISTEN` | `:8120` | HTTP address of the routes |
| `MESAIL_CALLS_MEDIA_LISTEN` | `:8121` | the one UDP address for all media |
| `MESAIL_CALLS_PUBLIC_IPS` | empty | the public IP addresses devices reach the UDP port on, comma separated; the answer offers these instead of the container's own address |

Logs are JSON lines on standard error with call and device ids only:
never media, keys, tokens or SDP.

## Build and test

```bash
cd edge/calls && go test ./...
```

```bash
docker build --target calls -t mesail-calls .
```

Pion and its dependencies are vendored. After a change to `go.mod` or to
the `kit` packages it uses (`oidc`, `transport`, `apierrors`), run
`go mod vendor` here; a test fails while the vendored kit copy differs
from `kit/`.
