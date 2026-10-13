package caldav

import (
	"context"
	"errors"

	"github.com/d31ma/caduceus/internal/accounts"
)

type Discoverer interface {
	Discover(context.Context, *accounts.CalDAVEndpoint, accounts.Credentials) (Discovery, error)
}

// Validator composes mail/contact validation with optional CalDAV discovery.
type Validator struct {
	base   accounts.Validator
	calDAV Discoverer
}

func NewValidator(base accounts.Validator, calDAV Discoverer) *Validator {
	return &Validator{base: base, calDAV: calDAV}
}

func (validator *Validator) Validate(ctx context.Context, request accounts.ValidationRequest) (accounts.Capabilities, error) {
	if validator == nil || validator.base == nil {
		return accounts.Capabilities{}, accounts.ErrUnsupported
	}
	capabilities, err := validator.base.Validate(ctx, request)
	if err != nil || request.CalDAV == nil {
		return capabilities, err
	}
	if validator.calDAV == nil {
		return accounts.Capabilities{}, accounts.ErrUnsupported
	}
	_, err = validator.calDAV.Discover(ctx, request.CalDAV, request.Credentials)
	if err != nil {
		switch {
		case errors.Is(err, ErrAuthentication):
			return accounts.Capabilities{}, accounts.ValidationFailure{Service: accounts.ValidationCalDAV, Code: accounts.ValidationAuthentication}
		case errors.Is(err, ErrUnsafeEndpoint):
			return accounts.Capabilities{}, accounts.ValidationFailure{Service: accounts.ValidationCalDAV, Code: accounts.ValidationConnection}
		default:
			return accounts.Capabilities{}, accounts.ValidationFailure{Service: accounts.ValidationCalDAV, Code: accounts.ValidationProtocol}
		}
	}
	capabilities.Calendar = true
	return capabilities, nil
}
