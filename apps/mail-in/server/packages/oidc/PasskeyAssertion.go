package oidc

import (
	"encoding/base64"
	"encoding/json"
)

type PasskeyAssertion struct {
	Response struct {
		UserHandle string `json:"userHandle"`
	} `json:"response"`
}

func (PasskeyAssertion) PersonId(credentialJson string) string {
	assertion := PasskeyAssertion{}
	_ = json.Unmarshal([]byte(credentialJson), &assertion)
	personId, _ := base64.RawURLEncoding.DecodeString(assertion.Response.UserHandle)
	return string(personId)
}
