# ADR 0012: Audio and video calls

- Status: accepted (2026-11-14); the owner answered Q8 to Q11 the same
  day (section 6)
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
9. **Size, scope and release** (the owner's answers). Every call kind is in
   the first release: calls of two and group calls. A call holds up to 50
   people (the whole group); at most 9 videos show at once, the person
   speaking first. Screen sharing is on computers (browsers and the
   desktop apps), one shared screen at a time; phones later.
10. **Recording on the recorder's own device only** (the owner's answer).
   - The recorder's device records the call as it is shown to the
     recorder: every person's audio mixed, and the main video (the person
     speaking, or the shared screen). The file is saved on that device
     only, through the save dialog; no server ever receives it.
   - Before recording starts, the recorder's device sends an encrypted
     call message "recording started by <name>" to every participant and
     waits until the call's other devices received it; if it cannot, it
     does not record. Every participant's app then shows a "Recording"
     banner with the recorder's name that cannot be closed while the
     recording runs; people who join later get the current state when
     they join. Stopping sends "recording stopped".
   - Limit, written in SPEC: Mesail cannot stop someone from recording
     with another tool (a second phone, a modified app); the notice comes
     from the honest Mesail app.
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
- **P7, recording on the device.** A `Recorder` in the `Media` client:
  record a set of local and remote tracks (TACHYON mixes the audio tracks
  into one and records one video track the app chooses, which it may
  switch during the recording, for example to the person speaking or the
  shared screen), pause, resume and stop; write the recording as it runs
  (not all in memory) to a file the person picks in a save dialog, behind
  the `files` grant (WebM or MP4, whichever the platform makes; MediaRecorder
  in browsers, AVAssetWriter on Apple, MediaMuxer on Android, Media
  Foundation on Windows); events for started, stopped, failed and the
  bytes written; `supported()` says where it is missing. The recording is
  never uploaded by TACHYON.
- **Linux:** calls, screen sharing and recording must work in the Linux
  desktop app before the first release (ADR 0011 Q13, TACHYON request P8).
- **P6 (later), native call screen.** Incoming calls through CallKit (iOS)
  and ConnectionService (Android), with VoIP push, so a call rings like a
  phone call when the app is closed.

## TACHYON's approved plan for P5 and P7 (2026-11-14)

- Media on Apple runs on the same web path inside WKWebView (macOS checked;
  iOS checked in TACHYON's first slice).
- **SFrame** (RFC 9605) with AES-128-GCM is TACHYON's transform. Video uses
  **VP8**; TACHYON keeps the VP8 codec header in clear so `calls` can
  switch simulcast layers. Audio uses Opus. The SFrame keys are P2
  `Crypto` key handles: the client turns each call's MLS exporter secret
  into a non-extractable key handle (ADR 0011 keeps the MLS secrets
  themselves in the client's sealed tables).
- The Recorder (P7) is MediaRecorder in the web view; native code only
  saves the file.
- Screen sharing on phones is not in TACHYON's first slices (the owner
  chose computers first). Linux comes through P8.
- TACHYON's order: M0 probes; M1 devices, capture and `media-view` (no
  `Peers` needed); M2 sending on `Peers` (after P1a); M3 SFrame (after
  P2a); M4 background audio on phones; M5 Recorder.

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

## 6. The owner's answers (2026-11-14)

- Q8 "Should audio and video calls be in Mesail's first release?": "All
  calls" (calls of two and group calls, all in the first release).
- Q9 "How many people can be in one call?": "Up to 50 (Recommended)" (at
  most 9 videos shown at once, the person speaking first).
- Q10 "Should people be able to share their screen in calls?": "Computers
  first (Recommended)" (browsers and desktop apps; phones later).
- Q11 "Can calls be recorded?": "On own device" (only on the recorder's
  own device, and everyone in the call sees a "Recording" notice).

## Consequences

- SPEC section 6 changes; SPEC gains call rules (AC-CL) after the owner's
  answers (slice G9).
- New slices G9 to G12 and CU in SLICES Wave 9.
- One more public port on-premise; OPERATIONS.md changes (G10).
