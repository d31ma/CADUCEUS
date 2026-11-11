type TicketRecord struct {
	ID             string `json:"id,omitempty"`
	TicketHash     string `json:"ticketHash"`
	DomainId       string `json:"domainId"`
	MemberId       string `json:"memberId"`
	ExpiresAt      string `json:"expiresAt"`
	TokenExpiresAt string `json:"tokenExpiresAt"`
	UsedAt         string `json:"usedAt"`
}
