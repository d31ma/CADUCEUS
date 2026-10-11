type SessionChanged struct {
	SignedIn     bool     `json:"signedIn"`
	Token        string   `json:"token"`
	Email        string   `json:"email"`
	Name         string   `json:"name"`
	DisplayName  string   `json:"displayName"`
	Role         string   `json:"role"`
	Domains      []string `json:"domains"`
	RefreshToken string   `json:"refreshToken"`
}
