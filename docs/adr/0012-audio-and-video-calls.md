# ADR 0012: Audio and video calls

- Status: accepted (2026-11-14) as the direction; the choices in section
  6 wait for the owner's answers
- Deciders: the owner ("can you make sure the messaging has audio/video
  capabilities"), architecture (the design)
- Builds on: ADR 0011 (chat goes device to device: device keys, MLS
  groups of up to 50, WebRTC, signalling on `realtime`, the TURN relay).
  Amends ADR 0007 (one more public port). Changes SPEC section 6 ("Voice
  and video calls, screen sharing" were out of scope).

## Context

SPEC section 6 lists "Voice and video calls, screen sharing" as out of
scope for the first release. On 2026-11-14 the owner asked that messaging
has audio and video. ADR 0011 already makes chat device to device and end
to end encrypted, with WebRTC (TACHYON request P1, data channels only),
signalling on `realtime`'s socket, a TURN relay and MLS groups of up to
50.

## Decision

1. **Calls belong to chat.** A call is started from a direct conversation
   or a group, by its members, inside one domain. No calls with email
   contacts or other domains (SPEC section 6 keeps chat inside a domain).
2. **End to end encrypted, always.** The servers carry encrypted media and
   never hold the keys. The media keys come from the conversation's MLS
   group (an MLS exporter secret per call), so a member removed from the
   group cannot join or decrypt later calls.
3. **Two people: device to device.** A call between two people goes
   directly between their devices over WebRTC (with the TURN relay when
   needed), like ADR 0011's chat.
4. **Three or more: through a forwarding server that cannot read the
   media.** A mesh of devices works only for a few people, so a group call
   goes through a selective forwarding unit (SFU): each device sends its
   media once, and the SFU forwards it to the others. Each frame is
   encrypted on the sending device with SFrame (RFC 9605) before WebRTC's
   own transport encryption, so the SFU forwards frames it cannot read.
   The SFU is a new small Go app, `calls`, built on Pion (a Go WebRTC
   library), signalled through `realtime` like everything else (no
   second signalling server). Slice G9 confirms Pion against a ready-made
   SFU (for example LiveKit) and records the result here.
5. **Signalling and ringing.** Starting, answering, declining, leaving and
   ending a call are messages on `realtime`'s socket between members'
   devices (G2's signalling, extended). An incoming call rings on online
   devices through `realtime` and on others through a push with no
   names ("Incoming call", `notify`); the app shows who calls after it
   decrypts the call invitation. A real phone call screen (CallKit on iOS,
   ConnectionService on Android) is later (TACHYON request P6).
6. **Call history in the chat.** "Call started", "Call ended (12 min)" and
   "Missed call" are encrypted chat messages that the caller's device
   adds to the conversation, so they sync and back up like any message.
   A missed call also gets a push with no names ("Missed call").
7. **Controls:** mute, camera on and off, choose the microphone, camera
   and speaker, leave, and (for groups) a grid that shows a limited
   number of videos at once with the speaker first. Calls keep working
   when the screen locks on phones only where the native shell allows it
   (TACHYON P5).
8. **No change to email.** Email has no calls.

## Requests to TACHYON

- **P5, audio and video.** An extension of P1 (`Peers`) and a `Media`
  client for Tac services in every service language:
  - new browser grants `microphone` and `screen`, and the `camera` grant
    of P4, each with a reason in `tac.application.permissions`; the native
    shells' permission texts (iOS and macOS `NSMicrophoneUsageDescription`
    and `NSCameraUsageDescription`, Android `RECORD_AUDIO` and `CAMERA`,
    Windows capability declarations);
  - list the microphones, cameras and speakers, choose one, and follow
    changes (a headset plugged in);
  - capture a microphone track and a camera track (with echo
    cancellation and noise suppression on by default), and, where the
    platform allows, a screen track; stop each one;
  - add, replace and remove audio and video tracks on a `Peers`
    connection, with renegotiation, simulcast encodings for video, and
    connection statistics (bitrate, packet loss);
  - an encoded-frame transform hook (WebRTC encoded transforms /
    `RTCRtpScriptTransform`) that runs in a worker with a key the app
    gives (from P2), so the app can encrypt and decrypt each frame with
    SFrame; where a platform has no such hook, `supported()` says so;
  - mute (stop sending audio), camera off (stop sending video), and a
    `media-view` component that shows a local or remote video track by id
    (Tac components hold no script, ADR 0030);
  - on phones, keep a call's audio running in the background (iOS audio
    session, Android foreground service).
- **P6 (later), native call screen.** Incoming calls through CallKit (iOS)
  and ConnectionService (Android), with VoIP push, so a call rings like a
  phone call when the app is closed.

## Servers and on-premise

- New app `calls` (the SFU), stateless for media, with a list of active
  calls; it accepts only devices that `realtime` has admitted to a call of
  their domain.
- On-premise it needs public media ports: one UDP port for all calls (Pion
  can share one UDP port across calls), with TCP and the TURN relay as the
  fallback (ADR 0011). This amends ADR 0007 once more.
- Bandwidth: a group video call of 50 people is heavy on the SFU's
  network; the grid shows a limited number of videos (section 6, Q9) and
  uses simulcast so phones receive smaller videos.

## 6. Choices for the owner

- Q8: are calls in the first release (all, or only calls of two)?
- Q9: the largest call, and how many videos show at once.
- Q10: screen sharing: where, or not at all.
- Q11: recording: none, or on the recorder's own device with a notice.

## Consequences

- SPEC section 6 changes; SPEC gains call rules (AC-CL) after the owner's
  answers (slice G9).
- New slices G9 to G12 and CU in SLICES Wave 9.
- One more public port on-premise; OPERATIONS.md changes (G10).
