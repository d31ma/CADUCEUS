package oidc

import (
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type VerifierFactory struct{}

func (factory VerifierFactory) FromEnvironment() TokenVerifier {
	settings, err := SettingsReader{}.FromEnvironment()
	return factory.chosen(settings, err)
}

func (factory VerifierFactory) chosen(settings Settings, err error) TokenVerifier {
	if err != nil {
		return RefusingVerifier{}
	}
	return factory.Jwks(settings)
}

func (factory VerifierFactory) Jwks(settings Settings) JwksVerifier {
	keys := CachedKeySource{
		Fetcher: HttpKeyFetcher{Client: factory.client()},
		Cache:   factory.cache(),
		Now:     time.Now,
	}
	return JwksVerifier{Settings: settings, Keys: keys}
}

func (factory VerifierFactory) client() *http.Client {
	return &http.Client{Timeout: factory.fetchTimeout()}
}

func (factory VerifierFactory) cache() JsonFileCache {
	return JsonFileCache{Folder: filepath.Join(os.TempDir(), factory.cacheFolderName())}
}

func (VerifierFactory) fetchTimeout() time.Duration {
	return 5 * time.Second
}

func (VerifierFactory) cacheFolderName() string {
	return "mesail-oidc"
}
