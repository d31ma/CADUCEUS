package oidc

import (
	"encoding/base64"
	"encoding/json"
)

type SessionIssuerFactory struct{}

func (factory SessionIssuerFactory) FromJson(document string) SessionIssuer {
	snapshot := ApplicationSnapshot{}
	_ = json.Unmarshal([]byte(document), &snapshot)
	key, keyErr := base64.StdEncoding.DecodeString(snapshot.Yon.Identity.Session.Key)
	_, settingsErr := SettingsReader{}.FromJson(document)
	problem := factory.complete(snapshot.Yon, len(key), keyErr, settingsErr)
	return SessionIssuer{Settings: snapshot.Yon.Identity, Keycloak: snapshot.Yon.Oidc.Keycloak, Admin: snapshot.Yon.Oidc.Admin, Sealer: SealedCookie{Key: key}, Client: VerifierFactory{}.client(), Problem: problem}
}

func (factory SessionIssuerFactory) complete(settings ApplicationSettings, keyLength int, keyErr error, settingsErr error) error {
	if keyErr != nil || settingsErr != nil || keyLength != factory.keyLength() || settings.Oidc.Keycloak == "" || settings.Identity.Client.Secret == "" {
		return NotConfigured{}
	}
	return nil
}

func (SessionIssuerFactory) keyLength() int {
	return 32
}
