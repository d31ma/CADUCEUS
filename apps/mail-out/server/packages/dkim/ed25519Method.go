package dkim

import (
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
)

type ed25519Method struct{}

func (ed25519Method) algorithm() string {
	return "ed25519-sha256"
}

func (ed25519Method) keyType() string {
	return "ed25519"
}

func (ed25519Method) check(public crypto.PublicKey) error {
	return nil
}

func (ed25519Method) publicKey(public crypto.PublicKey) (string, error) {
	return base64.StdEncoding.EncodeToString(public.(ed25519.PublicKey)), nil
}

func (ed25519Method) sign(signer crypto.Signer, digest []byte) ([]byte, error) {
	return signer.Sign(rand.Reader, digest, crypto.Hash(0))
}
