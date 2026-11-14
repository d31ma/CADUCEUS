package oidc

import (
	"crypto/rsa"
	"math/big"
)

type RsaKey struct{}

func (rsaKey RsaKey) From(key JsonWebKey) (*rsa.PublicKey, error) {
	err := rsaKey.kind(key)
	if err != nil {
		return nil, err
	}
	modulus, exponent, err := KeyMaterial{}.Decoded(key.Modulus, key.Exponent)
	if err != nil {
		return nil, err
	}
	publicKey := &rsa.PublicKey{N: new(big.Int).SetBytes(modulus), E: int(new(big.Int).SetBytes(exponent).Int64())}
	return publicKey, rsaKey.strong(publicKey)
}

func (RsaKey) kind(key JsonWebKey) error {
	if key.KeyType != "RSA" {
		return RefusedToken{Reason: "the signing key is not an RSA key"}
	}
	return nil
}

func (rsaKey RsaKey) strong(publicKey *rsa.PublicKey) error {
	if publicKey.N.BitLen() < rsaKey.minimumBits() {
		return RefusedToken{Reason: "the RSA signing key is shorter than 2048 bits"}
	}
	return nil
}

func (RsaKey) minimumBits() int {
	return 2048
}
