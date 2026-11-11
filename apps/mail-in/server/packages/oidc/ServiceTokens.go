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

func (tokens ServiceTokens) Authorization(domainId string) (string, error) {
	token, err := tokens.token(domainId)
	return BearerToken{}.prefix() + token, err
}

func (tokens ServiceTokens) token(domainId string) (string, error) {
	if tokens.Problem != nil {
		return "", tokens.Problem
	}
	source, err := tokens.source(domainId)
	if err != nil {
		return "", err
	}
	cached := CachedToken{}
	tokens.Cache.Load(source.CacheKey(), &cached)
	return tokens.cachedOrFetched(source, cached)
}

func (tokens ServiceTokens) source(domainId string) (ServiceTokenSource, error) {
	if tokens.Settings.ServiceTokens != "" {
		return IdentityServiceToken{Client: tokens.Client, Endpoint: tokens.Settings.ServiceTokens, Secret: tokens.Secret, DomainId: domainId}, nil
	}
	endpoint, err := IssuerPattern{Pattern: tokens.Settings.Keycloak}.Expand(domainId)
	keycloak := KeycloakClient{Client: tokens.Client, Endpoint: endpoint, ClientId: AuthorizedParties{}.Service(), ClientSecret: tokens.Secret}
	return KeycloakServiceToken{Keycloak: keycloak}, err
}

func (tokens ServiceTokens) cachedOrFetched(source ServiceTokenSource, cached CachedToken) (string, error) {
	if tokens.Now().Unix() < cached.ExpiresAt-tokens.marginSeconds() {
		return cached.AccessToken, nil
	}
	return tokens.fetched(source)
}

func (tokens ServiceTokens) fetched(source ServiceTokenSource) (string, error) {
	grant, err := source.Fetch()
	if err != nil {
		return "", err
	}
	_ = tokens.Cache.Store(source.CacheKey(), CachedToken{AccessToken: grant.AccessToken, ExpiresAt: tokens.Now().Unix() + grant.ExpiresIn})
	return grant.AccessToken, nil
}

func (ServiceTokens) marginSeconds() int64 {
	return 10
}
