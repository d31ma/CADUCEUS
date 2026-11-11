package dkim

import "crypto"

type method interface {
	algorithm() string
	keyType() string
	check(public crypto.PublicKey) error
	publicKey(public crypto.PublicKey) (string, error)
	sign(signer crypto.Signer, digest []byte) ([]byte, error)
}
