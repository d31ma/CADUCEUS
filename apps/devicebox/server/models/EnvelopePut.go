type EnvelopePut struct {
	EnvelopeId     string   `json:"envelopeId"`
	SenderDeviceId string   `json:"senderDeviceId"`
	ToDeviceIds    []string `json:"toDeviceIds"`
	Kind           string   `json:"kind"`
	Ciphertext     string   `json:"ciphertext"`
}
