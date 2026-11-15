package oidc

type PasswordCheck struct {
	Password    string
	OneTimeCode string
	Kept        bool
}
