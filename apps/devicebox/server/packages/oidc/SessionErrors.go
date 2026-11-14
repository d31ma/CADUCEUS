package oidc

import "errors"

type SessionErrors struct{}

func (SessionErrors) Normalized(err error) error {
	unavailable := KeycloakUnavailable{}
	switch {
	case err == nil:
		return nil
	case errors.Is(err, AccountLocked{}):
		return AccountLocked{}
	case errors.As(err, &unavailable) || errors.Is(err, NotConfigured{}):
		return SessionUnavailable{}
	}
	return SignInRefused{}
}
