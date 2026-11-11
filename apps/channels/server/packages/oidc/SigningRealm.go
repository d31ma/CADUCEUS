package oidc

import (
	"encoding/json"
	"time"
)

type SigningRealm struct {
	Name     string
	Settings Settings
	key      SigningKey
}

func (realm SigningRealm) Started(name string) (SigningRealm, error) {
	key, err := SigningKey{}.Generated()
	return SigningRealm{Name: name, Settings: realm.settings(), key: key}, err
}

func (realm SigningRealm) StartedMesail() (SigningRealm, error) {
	key, err := SigningKey{}.Generated()
	return SigningRealm{Settings: realm.mesailSettings(), key: key}, err
}

func (realm SigningRealm) ApplicationJson() (string, error) {
	raw, err := json.Marshal(ApplicationSnapshot{Yon: ApplicationSettings{Oidc: realm.Settings}})
	return string(raw), err
}

func (realm SigningRealm) Publish(now time.Time) error {
	jwksUrl, err := IssuerPattern{Pattern: realm.Settings.Jwks}.Resolved(realm.Name)
	if err != nil {
		return err
	}
	key, err := realm.key.PublicKey()
	if err != nil {
		return err
	}
	key.KeyId = realm.keyId()
	return realm.cache().Store(jwksUrl, CachedKeySet{FetchedAt: now.Unix(), Keys: KeySet{Keys: []JsonWebKey{key}}})
}

func (realm SigningRealm) Token(request TokenRequest) (string, error) {
	issuer, err := IssuerPattern{Pattern: realm.Settings.Issuer}.Resolved(realm.Name)
	if err != nil {
		return "", err
	}
	return JwtWriter{}.Signed(realm.key, realm.keyId(), realm.document(issuer, request))
}

func (realm SigningRealm) document(issuer string, request TokenRequest) ClaimsDocument {
	return ClaimsDocument{
		Issuer:          issuer,
		Subject:         request.Subject,
		Audience:        AudienceList{request.Audience},
		AuthorizedParty: request.AuthorizedParty,
		ExpiresAt:       request.ExpiresAt,
		TokenType:       ClaimRules{}.accessTokenType(),
		Name:            request.Name,
		Username:        request.Username,
		Email:           request.Email,
		Domain:          request.Domain,
		RealmAccess:     RealmAccess{Roles: request.RealmRoles},
	}
}

func (SigningRealm) settings() Settings {
	return Settings{
		Issuer:   "https://id.mesail.test/realms/{realm}",
		Jwks:     "https://id.mesail.test/realms/{realm}/protocol/openid-connect/certs",
		Audience: SigningRealm{}.audience(),
	}
}

func (realm SigningRealm) mesailSettings() Settings {
	return Settings{
		Issuer:        realm.mesailIssuer(),
		Jwks:          realm.mesailIssuer() + "/jwks",
		Audience:      "mesail-api",
		ServiceTokens: realm.mesailIssuer() + "/service-token",
	}
}

func (SigningRealm) mesailIssuer() string {
	return "https://mail.mesail.test/api/v1/identity"
}

func (SigningRealm) audience() string {
	return "mesail-api"
}

func (SigningRealm) cache() JsonFileCache {
	return VerifierFactory{}.cache()
}

func (SigningRealm) keyId() string {
	return "signing-realm"
}
