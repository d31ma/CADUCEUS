package oidc

type SetupExpired struct{}

func (SetupExpired) Error() string {
	return "the passkey setup is older than 10 minutes"
}

func (SetupExpired) PasskeyCode() string {
	return "setup_expired"
}
