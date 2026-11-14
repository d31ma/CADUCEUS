package fakerealtime

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"

	"mesail.dev/edge/calls/admission"
	"mesail.dev/packages/oidc"
)

type JoinKeys struct {
	key   oidc.SigningKey
	keyId string
}

func (JoinKeys) Generated() (JoinKeys, error) {
	key, err := oidc.SigningKey{}.Generated()
	if err != nil {
		return JoinKeys{}, err
	}
	keyId, err := key.KeyId()
	return JoinKeys{key: key, keyId: keyId}, err
}

func (keys JoinKeys) Set() (oidc.KeySet, error) {
	public, err := keys.key.PublicKey()
	return oidc.KeySet{Keys: []oidc.JsonWebKey{public}}, err
}

func (keys JoinKeys) Token(claims admission.JoinClaims) (string, error) {
	header, headerErr := json.Marshal(oidc.TokenHeader{Algorithm: "ES256", KeyId: keys.keyId, Type: "JWT"})
	payload, payloadErr := json.Marshal(claims)
	input := keys.encoded(header) + "." + keys.encoded(payload)
	signature, signErr := keys.key.Sign(input)
	return input + "." + keys.encoded(signature), errors.Join(headerErr, payloadErr, signErr)
}

func (keys JoinKeys) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	set, _ := keys.Set()
	writer.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(writer).Encode(set)
}

func (JoinKeys) encoded(raw []byte) string {
	return base64.RawURLEncoding.EncodeToString(raw)
}
