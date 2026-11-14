package devicebox

import "slices"

type PutCheck struct{}

func (check PutCheck) Checked(request PutRequest) (CheckedPut, error) {
	envelopeId, err := check.envelopeId(request.EnvelopeId)
	if err != nil {
		return CheckedPut{}, err
	}
	err = check.sender(request.SenderDeviceId)
	if err != nil {
		return CheckedPut{}, err
	}
	err = check.receivers(request.ToDeviceIds)
	if err != nil {
		return CheckedPut{}, err
	}
	err = check.kind(request.Kind)
	if err != nil {
		return CheckedPut{}, err
	}
	err = check.ciphertext(request.Ciphertext)
	put := CheckedPut{EnvelopeId: envelopeId, SenderDeviceId: request.SenderDeviceId, ToDeviceIds: request.ToDeviceIds, Kind: request.Kind, Ciphertext: request.Ciphertext}
	return put, err
}

func (PutCheck) envelopeId(text string) (string, error) {
	envelopeId, valid := Formats{}.EnvelopeId(text)
	if !valid {
		return "", InvalidRequest{}.EnvelopeId()
	}
	return envelopeId, nil
}

func (PutCheck) sender(deviceId string) error {
	if !(Formats{}).DeviceId(deviceId) {
		return InvalidRequest{}.SenderDevice()
	}
	return nil
}

func (check PutCheck) receivers(deviceIds []string) error {
	if len(deviceIds) < 1 || len(deviceIds) > (Limits{}).MaxDevices() {
		return InvalidRequest{}.DeviceCount()
	}
	return check.receiverIds(deviceIds)
}

func (check PutCheck) receiverIds(deviceIds []string) error {
	if slices.ContainsFunc(deviceIds, check.badDeviceId) || check.repeats(deviceIds) {
		return InvalidRequest{}.DeviceId()
	}
	return nil
}

func (PutCheck) badDeviceId(deviceId string) bool {
	return !Formats{}.DeviceId(deviceId)
}

func (PutCheck) repeats(deviceIds []string) bool {
	unique := slices.Clone(deviceIds)
	slices.Sort(unique)
	return len(slices.Compact(unique)) != len(deviceIds)
}

func (PutCheck) kind(kind string) error {
	if !slices.Contains(Formats{}.Kinds(), kind) {
		return InvalidRequest{}.Kind()
	}
	return nil
}

func (check PutCheck) ciphertext(text string) error {
	size, valid := Formats{}.DecodedSize(text)
	if !valid {
		return InvalidRequest{}.Ciphertext()
	}
	return check.size(size)
}

func (PutCheck) size(size int) error {
	if size > (Limits{}).MaxCiphertextBytes() {
		return TooLarge{}.Ciphertext()
	}
	return nil
}
