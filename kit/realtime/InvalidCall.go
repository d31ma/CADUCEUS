package realtime

type InvalidCall struct{}

func (InvalidCall) PairingIdInvalid() SignalRefusal {
	return SignalRefusal{Code: "invalid_request", Field: "pairingId", Text: "must be 16 to 64 letters, digits, - or _", ErrorNumber: 5032}
}

func (InvalidCall) UnknownMedia() SignalRefusal {
	return SignalRefusal{Code: "invalid_request", Field: "media", Text: "must be audio or video", ErrorNumber: 5034}
}

func (InvalidCall) RingCount() SignalRefusal {
	return SignalRefusal{Code: "invalid_request", Field: "toPersonIds", Text: "must name 1 to 49 people", ErrorNumber: 5038}
}

func (InvalidCall) InviteSize() SignalRefusal {
	return SignalRefusal{Code: "invalid_request", Field: "invite", Text: "must be base64url text of 1 to 16384 bytes", ErrorNumber: 5039}
}

func (InvalidCall) MissedCount() SignalRefusal {
	return SignalRefusal{Code: "invalid_request", Field: "missed", Text: "must name at most 49 people", ErrorNumber: 5040}
}
