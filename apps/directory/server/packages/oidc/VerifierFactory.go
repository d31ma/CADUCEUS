package oidc

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"mesail.dev/packages/transport"
)

type VerifierFactory struct{}

func (factory VerifierFactory) FromEnvironment() TokenVerifier {
	settings, err := SettingsReader{}.FromEnvironment()
	return factory.chosen(factory.byMode(settings), err)
}

func (factory VerifierFactory) byMode(settings Settings) TokenVerifier {
	if (IssuerPattern{Pattern: settings.Issuer}).HasPlaceholder() {
		return factory.Jwks(settings)
	}
	return factory.Mesail(settings)
}

func (VerifierFactory) chosen(verifier TokenVerifier, err error) TokenVerifier {
	if err != nil {
		return RefusingVerifier{}
	}
	return verifier
}

func (factory VerifierFactory) Jwks(settings Settings) JwksVerifier {
	return JwksVerifier{Settings: settings, Keys: factory.keys()}
}

func (factory VerifierFactory) Mesail(settings Settings) ExactIssuerVerifier {
	return ExactIssuerVerifier{Issuer: settings.Issuer, Jwks: settings.Jwks, Keys: factory.keys(), Rules: MesailClaimRules{Audience: settings.Audience}}
}

func (factory VerifierFactory) Keycloak(endpoint string) ExactIssuerVerifier {
	issuer := strings.TrimSuffix(endpoint, factory.protocolPath())
	return ExactIssuerVerifier{Issuer: issuer, Jwks: endpoint + factory.certificatesPath(), Keys: factory.keys(), Rules: ClaimRules{Audience: AuthorizedParties{}.Identity()}}
}

func (factory VerifierFactory) keys() CachedKeySource {
	return CachedKeySource{
		Fetcher: HttpKeyFetcher{Client: factory.client()},
		Cache:   factory.cache(),
		Now:     time.Now,
	}
}

func (factory VerifierFactory) client() *http.Client {
	return transport.SingleUseTransport{}.Client(factory.fetchTimeout())
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

func (VerifierFactory) protocolPath() string {
	return "/protocol/openid-connect"
}

func (VerifierFactory) certificatesPath() string {
	return "/certs"
}
