package oidc

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"time"
)

type SessionIssuerFactory struct{}

func (factory SessionIssuerFactory) Configured() SessionIssuer {
	return factory.FromJson(os.Getenv(SettingsReader{}.variable()))
}

func (factory SessionIssuerFactory) FromJson(document string) SessionIssuer {
	snapshot := ApplicationSnapshot{}
	_ = json.Unmarshal([]byte(document), &snapshot)
	key, keyErr := base64.StdEncoding.DecodeString(snapshot.Yon.Identity.Session.Key)
	settings, settingsErr := SettingsReader{}.FromJson(document)
	gate := NewGate(VerifierFactory{}.Jwks(settings), time.Now)
	problem := factory.complete(snapshot.Yon.Identity, len(key), keyErr, settingsErr)
	return SessionIssuer{Gate: gate, Settings: snapshot.Yon.Identity, Sealer: SealedCookie{Key: key}, Client: VerifierFactory{}.client(), Problem: problem}
}

func (factory SessionIssuerFactory) complete(settings SessionSettings, keyLength int, keyErr error, settingsErr error) error {
	if keyErr != nil || settingsErr != nil || keyLength != factory.keyLength() || settings.Keycloak == "" || settings.Client.Secret == "" {
		return NotConfigured{}
	}
	return nil
}

func (SessionIssuerFactory) keyLength() int {
	return 32
}
