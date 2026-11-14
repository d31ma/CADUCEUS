package oidc

import "errors"

type AccountErrors struct{}

func (AccountErrors) Normalized(err error) error {
	refused := AdminRefused{}
	var passkey PasskeyFailure
	switch {
	case errors.As(err, &refused):
		return refused
	case errors.As(err, &passkey):
		return passkey
	case errors.Is(err, InvalidCode{}):
		return InvalidCode{}
	}
	return SessionErrors{}.Normalized(err)
}
