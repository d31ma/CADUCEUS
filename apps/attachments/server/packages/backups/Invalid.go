package backups

type Invalid struct {
	Field       string
	Reason      string
	ErrorNumber int
}

func (Invalid) ExpectedVersion() Invalid {
	return Invalid{Field: "expectedVersion", Reason: "Give the backup version you saw last, 0 or more.", ErrorNumber: 3515}
}

func (Invalid) Size() Invalid {
	return Invalid{Field: "size", Reason: "A backup must have at least one byte.", ErrorNumber: 3516}
}

func (Invalid) KeySlots() Invalid {
	return Invalid{Field: "keySlots", Reason: "Give 1 to 10 key slots of at most 1 KiB each, of kind recovery or passkey, with base64url values.", ErrorNumber: 3517}
}

func (Invalid) DeviceId() Invalid {
	return Invalid{Field: "deviceId", Reason: "Give the id of the device that made the backup: 1 to 128 letters, digits, - or _.", ErrorNumber: 3518}
}

func (invalid Invalid) Error() string {
	return invalid.Field + ": " + invalid.Reason
}
