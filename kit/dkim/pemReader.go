package dkim

import (
	"crypto"
	"crypto/x509"
	"encoding/pem"
)

type pemReader struct{}

func (pemReader) pkcs1() string {
	return "RSA PRIVATE KEY"
}

func (pemReader) pkcs8() string {
	return "PRIVATE KEY"
}

func (pemReader) unreadable() UnsupportedKey {
	return UnsupportedKey{Reason: "the secret is not a readable PEM private key"}
}

func (reader pemReader) signer(data []byte) (crypto.Signer, error) {
	block, err := reader.decode(data)
	if err != nil {
		return nil, err
	}
	parsed, err := reader.parse(block)
	if err != nil {
		return nil, reader.unreadable()
	}
	return reader.asSigner(parsed)
}

func (reader pemReader) decode(data []byte) (*pem.Block, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, reader.unreadable()
	}
	return block, nil
}

func (reader pemReader) parse(block *pem.Block) (any, error) {
	switch block.Type {
	case reader.pkcs1():
		return x509.ParsePKCS1PrivateKey(block.Bytes)
	case reader.pkcs8():
		return x509.ParsePKCS8PrivateKey(block.Bytes)
	default:
		return nil, reader.unreadable()
	}
}

func (reader pemReader) asSigner(parsed any) (crypto.Signer, error) {
	signer, ok := parsed.(crypto.Signer)
	if !ok {
		return nil, reader.unreadable()
	}
	return signer, nil
}
