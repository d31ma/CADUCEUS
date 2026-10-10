type Principal struct {
	PrincipalId string  `json:"principalId"`
	TenantId    string  `json:"tenantId"`
	SessionId   string  `json:"sessionId"`
	Address     string  `json:"address"`
	Domain      string  `json:"domain"`
	Local       string  `json:"local"`
	Scope       string  `json:"scope"`
	ExpiresAt   float64 `json:"expiresAt"`
}
