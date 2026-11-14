package oidc

import (
	"crypto/rand"
	"time"
)

type TokenSigner struct {
	Issuer   string
	Audience string
	Current  SigningKey
	Previous []SigningKey
	Now      func() time.Time
	Problem  error
}

func (signer TokenSigner) Sign(token MesailToken) (IssuedToken, error) {
	return signer.sign(token)
}

func (signer TokenSigner) KeySet() (KeySet, error) {
	return signer.keySet()
}

func (signer TokenSigner) sign(token MesailToken) (IssuedToken, error) {
	if signer.Problem != nil {
		return IssuedToken{}, signer.Problem
	}
	keyId, err := signer.Current.KeyId()
	if err != nil {
		return IssuedToken{}, err
	}
	signed, err := JwtWriter{}.Signed(signer.Current, keyId, signer.document(token, rand.Text()))
	return IssuedToken{AccessToken: signed, ExpiresIn: signer.lifetimeSeconds()}, err
}

func (signer TokenSigner) keySet() (KeySet, error) {
	if signer.Problem != nil {
		return KeySet{}, signer.Problem
	}
	return signer.published(append([]SigningKey{signer.Current}, signer.Previous...))
}

func (signer TokenSigner) document(token MesailToken, tokenId string) ClaimsDocument {
	issuedAt := signer.Now().Unix()
	return ClaimsDocument{
		Issuer:          signer.Issuer,
		Subject:         token.Subject,
		Audience:        AudienceList{signer.Audience},
		AuthorizedParty: token.AuthorizedParty,
		ExpiresAt:       issuedAt + signer.lifetimeSeconds(),
		NotBefore:       issuedAt,
		IssuedAt:        issuedAt,
		TokenId:         tokenId,
		SessionId:       token.SessionId,
		Name:            token.Name,
		Username:        token.Username,
		Email:           token.Email,
		Domain:          token.Domain,
		RealmAccess:     RealmAccess{Roles: token.RealmRoles},
	}
}

func (TokenSigner) published(keys []SigningKey) (KeySet, error) {
	set := KeySet{Keys: []JsonWebKey{}}
	for _, key := range keys {
		public, err := key.PublicKey()
		if err != nil {
			return KeySet{}, err
		}
		set.Keys = append(set.Keys, public)
	}
	return set, nil
}

func (TokenSigner) lifetimeSeconds() int64 {
	return 60
}
