type SessionTokenAnswer struct {
	AccessToken  string  `json:"accessToken"`
	RefreshToken string  `json:"refreshToken"`
	IdToken      string  `json:"idToken"`
	ExpiresIn    float64 `json:"expiresIn"`
}
