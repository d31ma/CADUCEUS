package realtime

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"

	"mesail.dev/packages/oidc"
)

type CallSigner struct {
	Current  oidc.SigningKey
	Previous []oidc.SigningKey
	Now      func() time.Time
	Problem  error
}

func (signer CallSigner) Ready() error {
	return signer.Problem
}

func (signer CallSigner) Token(callId string, deviceId string, domainId string) (string, error) {
	err := signer.Ready()
	if err != nil {
		return "", err
	}
	keyId, err := signer.Current.KeyId()
	if err != nil {
		return "", err
	}
	return signer.signed(keyId, signer.claims(callId, deviceId, domainId))
}

func (signer CallSigner) KeySet() (oidc.KeySet, error) {
	err := signer.Ready()
	if err != nil {
		return oidc.KeySet{}, err
	}
	return oidc.TokenSigner{Current: signer.Current, Previous: signer.Previous}.KeySet()
}

func (signer CallSigner) claims(callId string, deviceId string, domainId string) JoinClaims {
	issuedAt := signer.Now().Unix()
	lifetime := int64(SignalLimits{}.JoinTokenLifetime() / time.Second)
	return JoinClaims{CallId: callId, DeviceId: deviceId, DomainId: domainId, Audience: signer.audience(), IssuedAt: issuedAt, ExpiresAt: issuedAt + lifetime}
}

func (signer CallSigner) signed(keyId string, claims JoinClaims) (string, error) {
	header, headerErr := json.Marshal(oidc.TokenHeader{Algorithm: signer.algorithm(), KeyId: keyId, Type: "JWT"})
	payload, payloadErr := json.Marshal(claims)
	input := signer.encoded(header) + "." + signer.encoded(payload)
	signature, signErr := signer.Current.Sign(input)
	return input + "." + signer.encoded(signature), errors.Join(headerErr, payloadErr, signErr)
}

func (CallSigner) encoded(raw []byte) string {
	return base64.RawURLEncoding.EncodeToString(raw)
}

func (CallSigner) audience() string {
	return "calls"
}

func (CallSigner) algorithm() string {
	return "ES256"
}
