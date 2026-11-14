package realtime

type CallRefusal struct{}

func (CallRefusal) CallFull() SignalRefusal {
	return SignalRefusal{Code: "call_full", Field: "callId", Text: "The call already has 50 people.", ErrorNumber: 5037}
}

func (CallRefusal) TooManyCalls() SignalRefusal {
	return SignalRefusal{Code: "rate_limited", Field: "", Text: "This device started 10 calls in the last minute. Wait a minute and try again.", ErrorNumber: 5041}
}

func (CallRefusal) SigningOff() SignalRefusal {
	return SignalRefusal{Code: "unavailable", Field: "", Text: "Group calls are not available right now.", ErrorNumber: 5042}
}

func (CallRefusal) DeviceNotLinked() SignalRefusal {
	return SignalRefusal{Code: "forbidden", Field: "deviceId", Text: "This device is not linked yet.", ErrorNumber: 5044}
}

func (CallRefusal) Unexpected() SignalRefusal {
	return SignalRefusal{Code: "unavailable", Field: "", Text: "Realtime is not available right now. Try again later.", ErrorNumber: 5045}
}
