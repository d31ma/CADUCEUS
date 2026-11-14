package realtime

import (
	"errors"
	"time"

	"mesail.dev/packages/oidc"
)

type CallSignerFactory struct{}

func (factory CallSignerFactory) FromSettings(settings CallKeySettings, now func() time.Time) CallSigner {
	current, currentErr := oidc.SigningKey{}.Parsed(settings.Key)
	previous, previousErr := factory.previous(settings.Previous)
	return CallSigner{Current: current, Previous: previous, Now: now, Problem: factory.problem(errors.Join(currentErr, previousErr))}
}

func (CallSignerFactory) previous(text string) ([]oidc.SigningKey, error) {
	if text == "" {
		return nil, nil
	}
	key, err := oidc.SigningKey{}.Parsed(text)
	return []oidc.SigningKey{key}, err
}

func (CallSignerFactory) problem(err error) error {
	if err != nil {
		return CallRefusal{}.SigningOff()
	}
	return nil
}
