package dkim

import (
	"crypto"
	"crypto/ed25519"
	"crypto/rsa"
)

type methods struct{}

func (all methods) forKey(key Key) (method, crypto.PublicKey, error) {
	public, err := all.publicOf(key.Signer)
	if err != nil {
		return nil, nil, err
	}
	chosen, err := all.choose(public)
	if err != nil {
		return nil, nil, err
	}
	return chosen, public, chosen.check(public)
}

func (methods) publicOf(signer crypto.Signer) (crypto.PublicKey, error) {
	if signer == nil {
		return nil, UnsupportedKey{Reason: "the key has no signer"}
	}
	return signer.Public(), nil
}

func (methods) choose(public crypto.PublicKey) (method, error) {
	switch public.(type) {
	case *rsa.PublicKey:
		return rsaMethod{}, nil
	case ed25519.PublicKey:
		return ed25519Method{}, nil
	default:
		return nil, UnsupportedKey{Reason: "only rsa and ed25519 keys can sign"}
	}
}
