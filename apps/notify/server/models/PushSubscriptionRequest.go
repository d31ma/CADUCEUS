type PushSubscriptionRequest struct {
	Endpoint  string `json:"endpoint"`
	PublicKey string `json:"p256dh"`
	Auth      string `json:"auth"`
}
