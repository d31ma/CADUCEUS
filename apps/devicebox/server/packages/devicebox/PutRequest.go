package devicebox

type PutRequest struct {
	EnvelopeId     string
	SenderDeviceId string
	ToDeviceIds    []string
	Kind           string
	Ciphertext     string
}
