package oidc

type PasskeyNotFound struct{}

func (PasskeyNotFound) Error() string {
	return "the person has no passkey with this id"
}

func (PasskeyNotFound) PasskeyCode() string {
	return "not_found"
}
