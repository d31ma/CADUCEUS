type DeliveryStatus struct {
	Recipient string `json:"recipient"`
	Channel   string `json:"channel"`
	Status    string `json:"status"`
	Detail    string `json:"detail,omitempty"`
}
