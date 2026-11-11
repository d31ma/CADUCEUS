package core

type Member struct {
	Id             string `json:"id,omitempty"`
	DomainId       string `json:"domainId"`
	MemberId       string `json:"memberId"`
	DisplayName    string `json:"displayName"`
	PrimaryAddress string `json:"primaryAddress"`
	LocalPart      string `json:"localPart"`
	State          string `json:"state"`
}
