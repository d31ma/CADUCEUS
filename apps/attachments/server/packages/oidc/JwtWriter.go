package oidc

import (
	"encoding/base64"
	"encoding/json"
	"errors"
)

type JwtWriter struct{}

func (writer JwtWriter) Signed(key SigningKey, keyId string, document ClaimsDocument) (string, error) {
	header, headerErr := json.Marshal(TokenHeader{Algorithm: SignatureCheck{}.ecdsaAlgorithm(), KeyId: keyId, Type: writer.tokenType()})
	claims, claimsErr := json.Marshal(document)
	input := writer.encoded(header) + "." + writer.encoded(claims)
	signature, signErr := key.Sign(input)
	return input + "." + writer.encoded(signature), errors.Join(headerErr, claimsErr, signErr)
}

func (JwtWriter) encoded(raw []byte) string {
	return base64.RawURLEncoding.EncodeToString(raw)
}

func (JwtWriter) tokenType() string {
	return "JWT"
}
