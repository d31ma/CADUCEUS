type SessionAccount struct {
	MemberId    string `json:"memberId"`
	DomainId    string `json:"domainId"`
	Address     string `json:"address"`
	DisplayName string `json:"displayName"`
	Initials    string `json:"initials"`
	Current     bool   `json:"current"`
}
