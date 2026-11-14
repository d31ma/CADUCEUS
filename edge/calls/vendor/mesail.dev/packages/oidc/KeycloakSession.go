package oidc

type KeycloakSession struct {
	PersonId           string
	SessionId          string
	Username           string
	DisplayName        string
	Operator           bool
	MustChangePassword bool
	RefreshToken       string
}
