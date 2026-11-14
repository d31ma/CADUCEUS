package oidc

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type ServiceTokensFactory struct{}

func (factory ServiceTokensFactory) Configured() ServiceTokens {
	return factory.FromJson(os.Getenv(SettingsReader{}.variable()))
}

func (factory ServiceTokensFactory) FromJson(document string) ServiceTokens {
	snapshot := ApplicationSnapshot{}
	_ = json.Unmarshal([]byte(document), &snapshot)
	settings := snapshot.Yon.Oidc
	secret := snapshot.Yon.Service.Client.Secret
	cache := JsonFileCache{Folder: filepath.Join(VerifierFactory{}.cache().Folder, factory.folderName())}
	return ServiceTokens{Settings: settings, Secret: secret, Client: VerifierFactory{}.client(), Cache: cache, Now: time.Now, Problem: factory.complete(settings, secret)}
}

func (ServiceTokensFactory) complete(settings Settings, secret string) error {
	if settings.ServiceTokens == "" || secret == "" {
		return NotConfigured{}
	}
	return nil
}

func (ServiceTokensFactory) folderName() string {
	return "tokens"
}
