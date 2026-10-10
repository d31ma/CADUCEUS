type TokenSet struct {
	AccessToken  string   `json:"accessToken"`
	Token        string   `json:"token"`
	RefreshToken string   `json:"refreshToken"`
	IDToken      string   `json:"idToken"`
	Address      string   `json:"address"`
	Email        string   `json:"email"`
	Domain       string   `json:"domain"`
	Domains      []string `json:"domains"`
	Name         string   `json:"name"`
	Role         string   `json:"role"`
}
