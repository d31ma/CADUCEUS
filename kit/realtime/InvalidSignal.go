package realtime

type InvalidSignal struct{}

func (InvalidSignal) DeviceMissing() SignalRefusal {
	return SignalRefusal{Code: "invalid_request", Field: "deviceId", Text: "the socket's auth must name this device", ErrorNumber: 5025}
}

func (InvalidSignal) TargetCount() SignalRefusal {
	return SignalRefusal{Code: "invalid_request", Field: "toDeviceIds", Text: "must name 1 to 200 devices", ErrorNumber: 5028}
}

func (InvalidSignal) UnknownKind() SignalRefusal {
	return SignalRefusal{Code: "invalid_request", Field: "kind", Text: "must be rtc, call or linking", ErrorNumber: 5029}
}

func (InvalidSignal) CiphertextSize() SignalRefusal {
	return SignalRefusal{Code: "invalid_request", Field: "ciphertext", Text: "must be base64url text of 1 to 65536 bytes", ErrorNumber: 5030}
}

func (InvalidSignal) RequestIdInvalid() SignalRefusal {
	return SignalRefusal{Code: "invalid_request", Field: "requestId", Text: "must be 1 to 64 characters", ErrorNumber: 5043}
}
