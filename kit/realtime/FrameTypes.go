package realtime

type FrameTypes struct{}

func (FrameTypes) Signal() string {
	return "signal"
}

func (FrameTypes) SignalResult() string {
	return "signal.result"
}

func (FrameTypes) PairingOpen() string {
	return "pairing.open"
}

func (FrameTypes) PairingOpened() string {
	return "pairing.opened"
}

func (FrameTypes) PairingJoin() string {
	return "pairing.join"
}

func (FrameTypes) PairingReady() string {
	return "pairing.ready"
}

func (FrameTypes) CallCreate() string {
	return "call.create"
}

func (FrameTypes) CallCreated() string {
	return "call.created"
}

func (FrameTypes) CallJoin() string {
	return "call.join"
}

func (FrameTypes) CallJoined() string {
	return "call.joined"
}

func (FrameTypes) CallRing() string {
	return "call.ring"
}

func (FrameTypes) CallRinging() string {
	return "call.ringing"
}

func (FrameTypes) CallRingEnd() string {
	return "call.ring.end"
}

func (FrameTypes) CallRingEnded() string {
	return "call.ring.ended"
}

func (FrameTypes) CallLeave() string {
	return "call.leave"
}

func (FrameTypes) CallLeft() string {
	return "call.left"
}

func (FrameTypes) Refused() string {
	return "refused"
}
