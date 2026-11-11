type SessionTokens struct {
	AccessToken  string  `json:"accessToken"`
	RefreshToken string  `json:"refreshToken"`
	IdToken      string  `json:"idToken"`
	ExpiresAtMs  float64 `json:"expiresAtMs"`
}
