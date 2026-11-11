type Me struct {
	MemberId       string   `json:"memberId"`
	DomainId       string   `json:"domainId"`
	DisplayName    string   `json:"displayName"`
	PrimaryAddress string   `json:"primaryAddress"`
	Roles          []string `json:"roles"`
}
