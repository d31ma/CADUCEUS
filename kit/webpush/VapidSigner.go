package webpush

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net/url"
	"time"
)

type VapidSigner struct {
	keys VapidKeys
	now  func() time.Time
}

func NewVapidSigner(keys VapidKeys, now func() time.Time) VapidSigner {
	return VapidSigner{keys: keys, now: now}
}

func (signer VapidSigner) Authorization(endpoint string) (string, error) {
	audience, err := signer.audience(endpoint)
	if err != nil {
		return "", err
	}
	key, err := signer.privateKey()
	if err != nil {
		return "", err
	}
	token, err := signer.token(key, audience)
	if err != nil {
		return "", err
	}
	return "vapid t=" + token + ", k=" + signer.keys.PublicKey, nil
}

func (VapidSigner) audience(endpoint string) (string, error) {
	address, err := url.Parse(endpoint)
	if err != nil || address.Host == "" {
		return "", RefusedEndpoint{Reason: "the endpoint is not an absolute URL"}
	}
	return address.Scheme + "://" + address.Host, nil
}

func (signer VapidSigner) privateKey() (*ecdsa.PrivateKey, error) {
	scalar, decodeError := Base64Text{}.Decode(signer.keys.PrivateKey)
	key, keyError := ecdsa.ParseRawPrivateKey(elliptic.P256(), scalar)
	return key, signer.configured(errors.Join(decodeError, keyError))
}

func (VapidSigner) configured(err error) error {
	if err != nil {
		return NotConfigured{}
	}
	return nil
}

func (signer VapidSigner) token(key *ecdsa.PrivateKey, audience string) (string, error) {
	expires := signer.now().Add(signer.lifetime()).Unix()
	claims, claimsError := json.Marshal(VapidClaims{Audience: audience, Expires: expires, Subject: signer.keys.Subject})
	unsigned := signer.header() + "." + Base64Text{}.Encode(claims)
	signature, signError := signer.sign(key, unsigned)
	return unsigned + "." + Base64Text{}.Encode(signature), errors.Join(claimsError, signError)
}

func (signer VapidSigner) sign(key *ecdsa.PrivateKey, unsigned string) ([]byte, error) {
	digest := sha256.Sum256([]byte(unsigned))
	first, second, err := ecdsa.Sign(rand.Reader, key, digest[:])
	if err != nil {
		return nil, err
	}
	signature := make([]byte, signer.scalarLength()*2)
	first.FillBytes(signature[:signer.scalarLength()])
	second.FillBytes(signature[signer.scalarLength():])
	return signature, nil
}

func (VapidSigner) header() string {
	return Base64Text{}.Encode([]byte(`{"typ":"JWT","alg":"ES256"}`))
}

func (VapidSigner) lifetime() time.Duration {
	return 12 * time.Hour
}

func (VapidSigner) scalarLength() int {
	return 32
}
