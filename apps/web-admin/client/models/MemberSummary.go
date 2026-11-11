type MemberSummary struct {
	MemberId       string `json:"memberId,omitempty"`
	Username       string `json:"username,omitempty"`
	DisplayName    string `json:"displayName"`
	PrimaryAddress string `json:"primaryAddress"`
	Admin          bool   `json:"admin,omitempty"`
	AccountManaged bool   `json:"accountManaged,omitempty"`
}
