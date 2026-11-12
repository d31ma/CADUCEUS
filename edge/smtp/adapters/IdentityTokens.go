package adapters

import (
	"context"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"mesail.dev/packages/oidc"
)

type IdentityTokens struct {
	settings TokenSettings
	client   *http.Client
	now      func() time.Time
	mutex    sync.Mutex
}

func NewIdentityTokens(settings TokenSettings, client *http.Client, now func() time.Time) *IdentityTokens {
	return &IdentityTokens{settings: settings, client: client, now: now}
}

func (tokens *IdentityTokens) Token(_ context.Context) (string, error) {
	tokens.mutex.Lock()
	defer tokens.mutex.Unlock()
	authorization, err := tokens.source(tokens.secret()).Authorization(oidc.InstallWide{}.DomainId())
	return tokens.tokenFrom(authorization, err)
}

func (tokens *IdentityTokens) secret() (string, error) {
	raw, err := os.ReadFile(tokens.settings.SecretFile)
	return strings.TrimSpace(string(raw)), err
}

func (tokens *IdentityTokens) source(secret string, problem error) oidc.ServiceTokens {
	return oidc.ServiceTokens{
		Settings: oidc.Settings{ServiceTokens: tokens.settings.Url},
		Secret:   secret,
		Client:   tokens.client,
		Cache:    oidc.JsonFileCache{Folder: tokens.settings.CacheFolder},
		Now:      tokens.now,
		Problem:  problem,
	}
}

func (*IdentityTokens) tokenFrom(authorization string, err error) (string, error) {
	if err != nil {
		return "", err
	}
	return oidc.BearerToken{}.FromHeader(authorization)
}
