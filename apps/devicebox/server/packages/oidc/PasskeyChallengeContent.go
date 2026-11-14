package oidc

type PasskeyChallengeContent struct {
	Challenge string `json:"challenge"`
	Subject   string `json:"subject,omitempty"`
	ExpiresAt int64  `json:"expiresAt"`
}
