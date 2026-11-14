package devicebox

type WaitingRow struct {
	Id             string `json:"id"`
	EnvelopeId     string `json:"envelopeId"`
	SenderDeviceId string `json:"senderDeviceId"`
	SenderPersonId string `json:"senderPersonId"`
	Kind           string `json:"kind"`
	StoredAt       string `json:"storedAt"`
	StoredUs       int64  `json:"storedUs"`
	BodyId         string `json:"bodyId"`
}
