package admission

type JoinClaims struct {
	CallId    string `json:"callId"`
	DeviceId  string `json:"deviceId"`
	DomainId  string `json:"domainId"`
	Audience  string `json:"aud"`
	ExpiresAt int64  `json:"exp"`
	IssuedAt  int64  `json:"iat"`
}
