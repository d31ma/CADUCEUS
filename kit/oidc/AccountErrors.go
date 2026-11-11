package oidc

import "errors"

type AccountErrors struct{}

func (AccountErrors) Normalized(err error) error {
	refused := AdminRefused{}
	switch {
	case errors.As(err, &refused):
		return refused
	case errors.Is(err, InvalidCode{}):
		return InvalidCode{}
	}
	return SessionErrors{}.Normalized(err)
}
