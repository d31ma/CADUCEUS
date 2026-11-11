package realtime

type Connection struct {
	ID           string `json:"id,omitempty"`
	ConnectionId string `json:"connectionId"`
	DomainId     string `json:"domainId"`
	MemberId     string `json:"memberId"`
	Kind         string `json:"kind"`
	ConnectedAt  string `json:"connectedAt"`
	ExpiresAt    string `json:"expiresAt"`
}
