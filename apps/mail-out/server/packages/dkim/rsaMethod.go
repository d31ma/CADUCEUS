package dkim

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"fmt"
)

type rsaMethod struct{}

func (rsaMethod) minimumBits() int {
	return 1024
}

func (rsaMethod) algorithm() string {
	return "rsa-sha256"
}

func (rsaMethod) keyType() string {
	return "rsa"
}

func (rsa256 rsaMethod) check(public crypto.PublicKey) error {
	bits := public.(*rsa.PublicKey).N.BitLen()
	if bits >= rsa256.minimumBits() {
		return nil
	}
	return UnsupportedKey{Reason: fmt.Sprintf("the rsa key has %d bits, at least %d are needed", bits, rsa256.minimumBits())}
}

func (rsaMethod) publicKey(public crypto.PublicKey) (string, error) {
	encoded, err := x509.MarshalPKIXPublicKey(public)
	if err != nil {
		return "", UnsupportedKey{Reason: "the rsa public key cannot be encoded"}
	}
	return base64.StdEncoding.EncodeToString(encoded), nil
}

func (rsaMethod) sign(signer crypto.Signer, digest []byte) ([]byte, error) {
	return signer.Sign(rand.Reader, digest, crypto.SHA256)
}
