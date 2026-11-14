package oidc

import (
	"encoding/json"
	"errors"
	"os"
	"time"
)

type TokenSignerFactory struct{}

func (factory TokenSignerFactory) Configured() TokenSigner {
	return factory.FromJson(os.Getenv(SettingsReader{}.variable()))
}

func (factory TokenSignerFactory) FromJson(document string) TokenSigner {
	snapshot := ApplicationSnapshot{}
	jsonErr := json.Unmarshal([]byte(document), &snapshot)
	signing := snapshot.Yon.Identity.Signing
	current, currentErr := SigningKey{}.Parsed(signing.Key)
	previous, previousErr := factory.previous(signing.Previous)
	problem := factory.complete(snapshot.Yon, errors.Join(jsonErr, currentErr, previousErr))
	return TokenSigner{Issuer: snapshot.Yon.Identity.Issuer, Audience: snapshot.Yon.Oidc.Audience, Current: current, Previous: previous, Now: time.Now, Problem: problem}
}

func (TokenSignerFactory) previous(text string) ([]SigningKey, error) {
	if text == "" {
		return nil, nil
	}
	key, err := SigningKey{}.Parsed(text)
	return []SigningKey{key}, err
}

func (TokenSignerFactory) complete(settings ApplicationSettings, err error) error {
	if err != nil || settings.Identity.Issuer == "" || settings.Oidc.Audience == "" {
		return NotConfigured{}
	}
	return nil
}
