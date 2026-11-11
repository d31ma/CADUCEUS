type MemberRow struct {
	MemberId       string `json:"memberId"`
	Name           string `json:"name"`
	Username       string `json:"username"`
	Address        string `json:"address"`
	Initials       string `json:"initials"`
	Admin          bool   `json:"admin"`
	AccountManaged bool   `json:"accountManaged"`
	You            bool   `json:"you"`
}
