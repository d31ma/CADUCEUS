package webpush

import (
	"crypto/ecdh"
	"io"
)

type VapidKeyGenerator struct{}

func (generator VapidKeyGenerator) Generate(random io.Reader) (VapidKeys, error) {
	scalar := make([]byte, generator.scalarLength())
	_, err := io.ReadFull(random, scalar)
	if err != nil {
		return VapidKeys{}, err
	}
	key, err := ecdh.P256().NewPrivateKey(scalar)
	if err != nil {
		return VapidKeys{}, err
	}
	return VapidKeys{PublicKey: Base64Text{}.Encode(key.PublicKey().Bytes()), PrivateKey: Base64Text{}.Encode(scalar)}, nil
}

func (VapidKeyGenerator) scalarLength() int {
	return 32
}
