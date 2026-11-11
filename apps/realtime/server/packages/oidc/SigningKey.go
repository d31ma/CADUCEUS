package oidc

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"strings"
)

type SigningKey struct {
	Private *ecdsa.PrivateKey
}

func (SigningKey) Generated() (SigningKey, error) {
	private, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	return SigningKey{Private: private}, err
}

func (key SigningKey) Parsed(text string) (SigningKey, error) {
	der, err := key.der(strings.TrimSpace(text))
	if err != nil {
		return SigningKey{}, err
	}
	parsed, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		return SigningKey{}, err
	}
	return key.p256(parsed)
}

func (key SigningKey) Text() (string, error) {
	der, err := x509.MarshalPKCS8PrivateKey(key.Private)
	return base64.StdEncoding.EncodeToString(der), err
}

func (key SigningKey) PublicKey() (JsonWebKey, error) {
	thumbprint, err := key.thumbprintInput()
	if err != nil {
		return JsonWebKey{}, err
	}
	keyId, err := key.thumbprint(thumbprint)
	return JsonWebKey{KeyType: thumbprint.KeyType, KeyId: keyId, Algorithm: SignatureCheck{}.ecdsaAlgorithm(), Use: key.signatureUse(), Curve: thumbprint.Curve, X: thumbprint.X, Y: thumbprint.Y}, err
}

func (key SigningKey) KeyId() (string, error) {
	thumbprint, err := key.thumbprintInput()
	if err != nil {
		return "", err
	}
	return key.thumbprint(thumbprint)
}

func (key SigningKey) Sign(input string) ([]byte, error) {
	digest := sha256.Sum256([]byte(input))
	r, s, err := ecdsa.Sign(rand.Reader, key.Private, digest[:])
	if err != nil {
		return nil, err
	}
	half := SignatureCheck{}.ecdsaHalfLength()
	return append(r.FillBytes(make([]byte, half)), s.FillBytes(make([]byte, half))...), nil
}

func (key SigningKey) thumbprintInput() (KeyThumbprintInput, error) {
	point, err := key.Private.PublicKey.Bytes()
	if err != nil {
		return KeyThumbprintInput{}, err
	}
	half := SignatureCheck{}.ecdsaHalfLength()
	return KeyThumbprintInput{Curve: "P-256", KeyType: "EC", X: key.encoded(point[1 : half+1]), Y: key.encoded(point[half+1:])}, nil
}

func (key SigningKey) thumbprint(input KeyThumbprintInput) (string, error) {
	raw, err := json.Marshal(input)
	digest := sha256.Sum256(raw)
	return key.encoded(digest[:]), err
}

func (SigningKey) der(text string) ([]byte, error) {
	block, _ := pem.Decode([]byte(text))
	if block != nil {
		return block.Bytes, nil
	}
	return base64.StdEncoding.DecodeString(text)
}

func (SigningKey) p256(parsed any) (SigningKey, error) {
	private, isEcdsa := parsed.(*ecdsa.PrivateKey)
	if !isEcdsa || private.Curve != elliptic.P256() {
		return SigningKey{}, RefusedToken{Reason: "the signing key is not a P-256 key"}
	}
	return SigningKey{Private: private}, nil
}

func (SigningKey) encoded(raw []byte) string {
	return base64.RawURLEncoding.EncodeToString(raw)
}

func (SigningKey) signatureUse() string {
	return "sig"
}
