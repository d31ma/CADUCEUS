package devicebox

type WaitingEnvelope struct {
	EnvelopeId     string `json:"envelopeId"`
	SenderDeviceId string `json:"senderDeviceId"`
	SenderPersonId string `json:"senderPersonId"`
	Kind           string `json:"kind"`
	Ciphertext     string `json:"ciphertext"`
	StoredAt       string `json:"storedAt"`
}
