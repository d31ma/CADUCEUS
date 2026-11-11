package oidc

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
)

type SealedCookie struct {
	Key []byte
}

func (sealed SealedCookie) Seal(value string, name string) (string, error) {
	aead, err := sealed.aead()
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	_, err = rand.Read(nonce)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(aead.Seal(nonce, nonce, []byte(value), []byte(name))), nil
}

func (sealed SealedCookie) Open(cookie string, name string) (string, error) {
	aead, err := sealed.aead()
	if err != nil {
		return "", err
	}
	raw, err := base64.RawURLEncoding.DecodeString(cookie)
	if err != nil {
		return "", err
	}
	nonce, ciphertext, err := sealed.split(raw, aead.NonceSize())
	if err != nil {
		return "", err
	}
	plain, err := aead.Open(nil, nonce, ciphertext, []byte(name))
	return string(plain), err
}

func (sealed SealedCookie) aead() (cipher.AEAD, error) {
	block, err := aes.NewCipher(sealed.Key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func (SealedCookie) split(raw []byte, nonceSize int) ([]byte, []byte, error) {
	if len(raw) < nonceSize {
		return nil, nil, RefusedToken{Reason: "the sealed cookie is too short"}
	}
	return raw[:nonceSize], raw[nonceSize:], nil
}
