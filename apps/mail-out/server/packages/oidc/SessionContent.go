package oidc

type SessionContent struct {
	Domain       string `json:"domain"`
	RefreshToken string `json:"refreshToken"`
}
