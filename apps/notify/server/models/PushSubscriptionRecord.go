type PushSubscriptionRecord struct {
	Id        string `json:"id,omitempty"`
	DomainId  string `json:"domainId"`
	MemberId  string `json:"memberId"`
	Platform  string `json:"platform"`
	Endpoint  string `json:"endpoint"`
	P256dh    string `json:"p256dh"`
	Auth      string `json:"auth"`
	CreatedAt string `json:"createdAt"`
}
