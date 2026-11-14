package devicebox

type CheckedPut struct {
	EnvelopeId     string
	SenderDeviceId string
	ToDeviceIds    []string
	Kind           string
	Ciphertext     string
}

func (put CheckedPut) Devices() []string {
	return append([]string{put.SenderDeviceId}, put.ToDeviceIds...)
}
