package oidc

type TooManyPasskeys struct{}

func (TooManyPasskeys) Error() string {
	return "the person already has 10 passkeys"
}

func (TooManyPasskeys) PasskeyCode() string {
	return "too_many"
}
