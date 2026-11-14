package oidc

import "time"

type ExactIssuerVerifier struct {
	Issuer string
	Jwks   string
	Keys   KeySource
	Rules  ClaimChecker
}

func (verifier ExactIssuerVerifier) Verify(rawToken string, now time.Time) (Claims, error) {
	token, err := TokenParser{}.Parse(rawToken)
	if err != nil {
		return Claims{}, err
	}
	err = verifier.issuer(token.Claims)
	if err != nil {
		return Claims{}, err
	}
	key, err := verifier.Keys.Key(verifier.Jwks, token.Header.KeyId)
	if err != nil {
		return Claims{}, err
	}
	err = SignatureCheck{}.Verify(token, key)
	if err != nil {
		return Claims{}, err
	}
	return token.Claims, verifier.Rules.Check(token.Claims, now)
}

func (verifier ExactIssuerVerifier) issuer(claims Claims) error {
	if verifier.Issuer == "" || claims.Issuer != verifier.Issuer {
		return UnknownIssuer{Issuer: claims.Issuer}
	}
	return nil
}
