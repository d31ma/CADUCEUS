package oidc

import (
	"crypto/ecdsa"
	"crypto/elliptic"
)

type EcdsaKey struct{}

func (ecdsaKey EcdsaKey) From(key JsonWebKey) (*ecdsa.PublicKey, error) {
	err := ecdsaKey.kind(key)
	if err != nil {
		return nil, err
	}
	x, y, err := KeyMaterial{}.Decoded(key.X, key.Y)
	if err != nil {
		return nil, err
	}
	point := append(append([]byte{ecdsaKey.uncompressedPrefix()}, x...), y...)
	return ecdsa.ParseUncompressedPublicKey(elliptic.P256(), point)
}

func (EcdsaKey) kind(key JsonWebKey) error {
	if key.KeyType != "EC" || key.Curve != "P-256" {
		return RefusedToken{Reason: "the signing key is not a P-256 key"}
	}
	return nil
}

func (EcdsaKey) uncompressedPrefix() byte {
	return 4
}
