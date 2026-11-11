package oidc

import "time"

type JwksVerifier struct {
	Settings Settings
	Keys     KeySource
}

func (verifier JwksVerifier) Verify(rawToken string, now time.Time) (Claims, error) {
	token, err := TokenParser{}.Parse(rawToken)
	if err != nil {
		return Claims{}, err
	}
	realm, err := IssuerPattern{Pattern: verifier.Settings.Issuer}.Realm(token.Claims.Issuer)
	if err != nil {
		return Claims{}, err
	}
	jwksUrl, err := IssuerPattern{Pattern: verifier.Settings.Jwks}.Expand(realm)
	if err != nil {
		return Claims{}, err
	}
	key, err := verifier.Keys.Key(jwksUrl, token.Header.KeyId)
	if err != nil {
		return Claims{}, err
	}
	err = SignatureCheck{}.Verify(token, key)
	if err != nil {
		return Claims{}, err
	}
	claims := token.Claims
	claims.Domain = realm
	return claims, ClaimRules{Audience: verifier.Settings.Audience}.Check(claims, now)
}
