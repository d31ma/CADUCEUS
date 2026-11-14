package realtime

type MissingTarget struct{}

func (MissingTarget) TargetNotFound() SignalRefusal {
	return SignalRefusal{Code: "not_found", Field: "toDeviceIds", Text: "Nothing was found.", ErrorNumber: 5031}
}

func (MissingTarget) PairingNotFound() SignalRefusal {
	return SignalRefusal{Code: "not_found", Field: "pairingId", Text: "Nothing was found.", ErrorNumber: 5033}
}

func (MissingTarget) CallNotFound() SignalRefusal {
	return SignalRefusal{Code: "not_found", Field: "callId", Text: "Nothing was found.", ErrorNumber: 5035}
}

func (MissingTarget) NotInCall() SignalRefusal {
	return SignalRefusal{Code: "not_found", Field: "callId", Text: "Nothing was found.", ErrorNumber: 5036}
}
