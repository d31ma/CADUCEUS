package carddav

import (
	"context"
	"errors"

	"github.com/d31ma/caduceus/internal/accounts"
)

type Discoverer interface {
	Discover(context.Context, *accounts.CardDAVEndpoint, accounts.Credentials) (Discovery, error)
}

// Validator composes mail validation with optional CardDAV discovery. The
// account is persisted only after both requested capabilities authenticate.
type Validator struct {
	mail    accounts.Validator
	cardDAV Discoverer
}

func NewValidator(mail accounts.Validator, cardDAV Discoverer) *Validator {
	return &Validator{mail: mail, cardDAV: cardDAV}
}

func (validator *Validator) Validate(ctx context.Context, request accounts.ValidationRequest) (accounts.Capabilities, error) {
	if validator == nil || validator.mail == nil {
		return accounts.Capabilities{}, accounts.ErrUnsupported
	}
	capabilities, err := validator.mail.Validate(ctx, request)
	if err != nil || request.CardDAV == nil {
		return capabilities, err
	}
	if validator.cardDAV == nil {
		return accounts.Capabilities{}, accounts.ErrUnsupported
	}
	_, err = validator.cardDAV.Discover(ctx, request.CardDAV, request.Credentials)
	if err != nil {
		switch {
		case errors.Is(err, ErrAuthentication):
			return accounts.Capabilities{}, accounts.ValidationFailure{Service: accounts.ValidationCardDAV, Code: accounts.ValidationAuthentication}
		case errors.Is(err, ErrUnsafeEndpoint):
			return accounts.Capabilities{}, accounts.ValidationFailure{Service: accounts.ValidationCardDAV, Code: accounts.ValidationConnection}
		default:
			return accounts.Capabilities{}, accounts.ValidationFailure{Service: accounts.ValidationCardDAV, Code: accounts.ValidationProtocol}
		}
	}
	capabilities.Contacts = true
	return capabilities, nil
}
