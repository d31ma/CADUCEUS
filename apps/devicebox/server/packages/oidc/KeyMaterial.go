package oidc

import "encoding/base64"

type KeyMaterial struct{}

func (KeyMaterial) Decoded(first string, second string) ([]byte, []byte, error) {
	firstBytes, err := base64.RawURLEncoding.DecodeString(first)
	if err != nil {
		return nil, nil, err
	}
	secondBytes, err := base64.RawURLEncoding.DecodeString(second)
	return firstBytes, secondBytes, err
}
