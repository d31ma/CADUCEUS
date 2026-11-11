package oidc

import (
	"net/http"
	"time"
)

type ServiceTokens struct {
	Settings Settings
	Secret   string
	Client   *http.Client
	Cache    JsonFileCache
	Now      func() time.Time
	Problem  error
}

func (tokens ServiceTokens) Authorization(realm string) (string, error) {
	token, err := tokens.token(realm)
	return BearerToken{}.prefix() + token, err
}

func (tokens ServiceTokens) token(realm string) (string, error) {
	if tokens.Problem != nil {
		return "", tokens.Problem
	}
	endpoint, err := IssuerPattern{Pattern: tokens.Settings.Keycloak}.Expand(realm)
	if err != nil {
		return "", err
	}
	cached := CachedToken{}
	tokens.Cache.Load(endpoint, &cached)
	return tokens.cachedOrFetched(endpoint, cached)
}

func (tokens ServiceTokens) cachedOrFetched(endpoint string, cached CachedToken) (string, error) {
	if tokens.Now().Unix() < cached.ExpiresAt-tokens.marginSeconds() {
		return cached.AccessToken, nil
	}
	return tokens.fetched(endpoint)
}

func (tokens ServiceTokens) fetched(endpoint string) (string, error) {
	keycloak := KeycloakClient{Client: tokens.Client, Endpoint: endpoint, ClientId: tokens.clientId(), ClientSecret: tokens.Secret}
	grant, err := keycloak.ClientCredentials()
	if err != nil {
		return "", err
	}
	_ = tokens.Cache.Store(endpoint, CachedToken{AccessToken: grant.AccessToken, ExpiresAt: tokens.Now().Unix() + grant.ExpiresIn})
	return grant.AccessToken, nil
}

func (ServiceTokens) clientId() string {
	return "mesail-service"
}

func (ServiceTokens) marginSeconds() int64 {
	return 10
}
