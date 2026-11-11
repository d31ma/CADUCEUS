package oidc

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/sha256"
	"math/big"
	"slices"
)

type SignatureCheck struct{}

func (check SignatureCheck) Supports(algorithm string) bool {
	return slices.Contains([]string{check.rsaAlgorithm(), check.ecdsaAlgorithm()}, algorithm)
}

func (check SignatureCheck) Verify(token SignedToken, key JsonWebKey) error {
	digest := sha256.Sum256([]byte(token.SigningInput))
	return check.byAlgorithm(token, key, digest[:])
}

func (check SignatureCheck) byAlgorithm(token SignedToken, key JsonWebKey, digest []byte) error {
	switch token.Header.Algorithm {
	case check.rsaAlgorithm():
		return check.rsa(key, digest, token.Signature)
	case check.ecdsaAlgorithm():
		return check.ecdsa(key, digest, token.Signature)
	}
	return RefusedToken{Reason: "the signing algorithm is not RS256 or ES256"}
}

func (check SignatureCheck) rsa(key JsonWebKey, digest []byte, signature []byte) error {
	publicKey, err := RsaKey{}.From(key)
	if err != nil {
		return err
	}
	return check.matched(rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, digest, signature) == nil)
}

func (check SignatureCheck) ecdsa(key JsonWebKey, digest []byte, signature []byte) error {
	publicKey, err := EcdsaKey{}.From(key)
	if err != nil {
		return err
	}
	return check.matched(check.ecdsaValid(publicKey, digest, signature))
}

func (check SignatureCheck) ecdsaValid(publicKey *ecdsa.PublicKey, digest []byte, signature []byte) bool {
	half := check.ecdsaHalfLength()
	if len(signature) != 2*half {
		return false
	}
	return ecdsa.Verify(publicKey, digest, new(big.Int).SetBytes(signature[:half]), new(big.Int).SetBytes(signature[half:]))
}

func (SignatureCheck) matched(valid bool) error {
	if !valid {
		return RefusedToken{Reason: "the signature does not match"}
	}
	return nil
}

func (SignatureCheck) rsaAlgorithm() string {
	return "RS256"
}

func (SignatureCheck) ecdsaAlgorithm() string {
	return "ES256"
}

func (SignatureCheck) ecdsaHalfLength() int {
	return 32
}
