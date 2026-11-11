package mailauth

import (
	"crypto"
	"crypto/rsa"
)

type rsaKey struct {
	public *rsa.PublicKey
}

func (rsaKey) kind() string {
	return dkimAlgorithms{}.rsa()
}

func (key rsaKey) verify(digest []byte, signature []byte) bool {
	return rsa.VerifyPKCS1v15(key.public, crypto.SHA256, digest, signature) == nil
}
