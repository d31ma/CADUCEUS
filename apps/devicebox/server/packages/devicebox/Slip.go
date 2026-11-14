package devicebox

type Slip struct {
	Id             string          `json:"id,omitempty"`
	EnvelopeId     string          `json:"envelopeId"`
	DomainId       string          `json:"domainId"`
	SenderDeviceId string          `json:"senderDeviceId"`
	SenderPersonId string          `json:"senderPersonId"`
	Kind           string          `json:"kind"`
	StoredAt       string          `json:"storedAt"`
	StoredUs       int64           `json:"storedUs"`
	ExpiresUs      int64           `json:"expiresUs"`
	BodyId         string          `json:"bodyId"`
	ContentHash    string          `json:"contentHash"`
	Waiting        map[string]bool `json:"waiting"`
}
