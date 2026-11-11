package core

type DeliveryRecord struct {
	Recipient string `json:"recipient"`
	MemberId  string `json:"memberId"`
	Channel   string `json:"channel"`
	Role      string `json:"role"`
	Status    string `json:"status"`
	Detail    string `json:"detail"`
}
