package oidc

type SessionContent struct {
	Domain       string `json:"domain"`
	RefreshToken string `json:"refreshToken"`
	PersonId     string `json:"personId,omitempty"`
	Kept         bool   `json:"kept,omitempty"`
	SignedInAt   string `json:"signedInAt,omitempty"`
}
