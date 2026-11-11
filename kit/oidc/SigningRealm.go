package oidc

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"time"
)

type SigningRealm struct {
	Name     string
	Settings Settings
	key      *ecdsa.PrivateKey
}

func (realm SigningRealm) Started(name string) (SigningRealm, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return SigningRealm{}, err
	}
	return SigningRealm{Name: name, Settings: realm.settings(), key: key}, nil
}

func (realm SigningRealm) ApplicationJson() (string, error) {
	raw, err := json.Marshal(ApplicationSnapshot{Yon: ApplicationSettings{Oidc: realm.Settings}})
	return string(raw), err
}

func (realm SigningRealm) Publish(now time.Time) error {
	jwksUrl, err := IssuerPattern{Pattern: realm.Settings.Jwks}.Expand(realm.Name)
	if err != nil {
		return err
	}
	point, err := realm.key.PublicKey.Bytes()
	if err != nil {
		return err
	}
	key := JsonWebKey{KeyType: "EC", KeyId: realm.keyId(), Algorithm: "ES256", Use: "sig", Curve: "P-256", X: realm.encoded(point[1:33]), Y: realm.encoded(point[33:])}
	return realm.cache().Store(jwksUrl, CachedKeySet{FetchedAt: now.Unix(), Keys: KeySet{Keys: []JsonWebKey{key}}})
}

func (realm SigningRealm) Token(request TokenRequest) (string, error) {
	issuer, err := IssuerPattern{Pattern: realm.Settings.Issuer}.Expand(realm.Name)
	if err != nil {
		return "", err
	}
	header, err := json.Marshal(TokenHeader{Algorithm: "ES256", KeyId: realm.keyId(), Type: "JWT"})
	if err != nil {
		return "", err
	}
	claims, err := json.Marshal(realm.document(issuer, request))
	if err != nil {
		return "", err
	}
	input := realm.encoded(header) + "." + realm.encoded(claims)
	signature, err := realm.signature(input)
	return input + "." + realm.encoded(signature), err
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
		Email:           request.Email,
		RealmAccess:     RealmAccess{Roles: request.RealmRoles},
	}
}

func (realm SigningRealm) signature(input string) ([]byte, error) {
	digest := sha256.Sum256([]byte(input))
	r, s, err := ecdsa.Sign(rand.Reader, realm.key, digest[:])
	if err != nil {
		return nil, err
	}
	half := SignatureCheck{}.ecdsaHalfLength()
	return append(r.FillBytes(make([]byte, half)), s.FillBytes(make([]byte, half))...), nil
}

func (SigningRealm) settings() Settings {
	return Settings{
		Issuer:   "https://id.mesail.test/realms/{realm}",
		Jwks:     "https://id.mesail.test/realms/{realm}/protocol/openid-connect/certs",
		Audience: "mesail-api",
	}
}

func (SigningRealm) cache() FileKeyCache {
	return VerifierFactory{}.cache()
}

func (SigningRealm) encoded(raw []byte) string {
	return base64.RawURLEncoding.EncodeToString(raw)
}

func (SigningRealm) keyId() string {
	return "signing-realm"
}
