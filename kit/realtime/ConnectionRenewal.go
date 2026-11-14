package realtime

type ConnectionRenewal struct {
	ExpiresAt string `json:"expiresAt"`
	Linked    bool   `json:"linked,omitempty"`
	CheckedAt string `json:"deviceCheckedAt,omitempty"`
}
