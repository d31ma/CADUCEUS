package oidc

import (
	"cmp"
	"encoding/json"
	"time"
)

type SigningRealm struct {
	Domain   string
	Settings Settings
	key      SigningKey
}

func (realm SigningRealm) StartedMesail() (SigningRealm, error) {
	key, err := SigningKey{}.Generated()
	return SigningRealm{Settings: realm.mesailSettings(), key: key}, err
}

func (realm SigningRealm) ForDomain(domainId string) SigningRealm {
	realm.Domain = domainId
	return realm
}

func (realm SigningRealm) ApplicationJson() (string, error) {
	raw, err := json.Marshal(ApplicationSnapshot{Yon: ApplicationSettings{Oidc: realm.Settings}})
	return string(raw), err
}

func (realm SigningRealm) Publish(now time.Time) error {
	key, err := realm.key.PublicKey()
	if err != nil {
		return err
	}
	key.KeyId = realm.keyId()
	return realm.cache().Store(realm.Settings.Jwks, CachedKeySet{FetchedAt: now.Unix(), Keys: KeySet{Keys: []JsonWebKey{key}}})
}

func (realm SigningRealm) Token(request TokenRequest) (string, error) {
	return JwtWriter{}.Signed(realm.key, realm.keyId(), realm.document(realm.Settings.Issuer, realm.withDomain(request)))
}

func (realm SigningRealm) withDomain(request TokenRequest) TokenRequest {
	request.Domain = cmp.Or(request.Domain, realm.Domain)
	return request
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

func (realm SigningRealm) mesailSettings() Settings {
	return Settings{
		Issuer:        realm.mesailIssuer(),
		Jwks:          realm.mesailIssuer() + "/jwks",
		Audience:      realm.audience(),
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
