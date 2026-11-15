package events

type MemberUpserted struct {
	MemberId       string `json:"memberId"`
	DisplayName    string `json:"displayName"`
	PrimaryAddress string `json:"primaryAddress"`
	State          string `json:"state"`
}
