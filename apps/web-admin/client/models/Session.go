type Session struct {
	Me          Me      `json:"me"`
	AccessToken string  `json:"accessToken"`
	ExpiresIn   float64 `json:"expiresIn"`
	SessionKey  string  `json:"sessionKey"`
}
