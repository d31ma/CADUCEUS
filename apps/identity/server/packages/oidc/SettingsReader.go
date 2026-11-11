package oidc

import (
	"encoding/json"
	"os"
)

type SettingsReader struct{}

func (reader SettingsReader) FromEnvironment() (Settings, error) {
	return reader.FromJson(os.Getenv(reader.variable()))
}

func (reader SettingsReader) FromJson(document string) (Settings, error) {
	snapshot := ApplicationSnapshot{}
	err := json.Unmarshal([]byte(document), &snapshot)
	return snapshot.Yon.Oidc, reader.complete(snapshot.Yon.Oidc, err)
}

func (SettingsReader) complete(settings Settings, err error) error {
	if err != nil || settings.Issuer == "" || settings.Jwks == "" || settings.Audience == "" {
		return NotConfigured{}
	}
	return nil
}

func (SettingsReader) variable() string {
	return "YON_APPLICATION_JSON"
}
