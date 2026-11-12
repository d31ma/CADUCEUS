package oidc

type InvalidPasskey struct{}

func (InvalidPasskey) Error() string {
	return "the passkey could not be added"
}

func (InvalidPasskey) PasskeyCode() string {
	return "invalid_passkey"
}
