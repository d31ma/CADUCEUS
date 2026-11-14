package oidc

type InvalidLabel struct{}

func (InvalidLabel) Error() string {
	return "a passkey label is 1 to 64 characters"
}

func (InvalidLabel) PasskeyCode() string {
	return "invalid_request"
}
