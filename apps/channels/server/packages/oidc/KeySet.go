package oidc

import "slices"

type KeySet struct {
	Keys []JsonWebKey `json:"keys"`
}

func (set KeySet) Find(keyId string) (JsonWebKey, error) {
	return set.at(set.position(keyId))
}

func (set KeySet) Has(keyId string) bool {
	return set.position(keyId) >= 0
}

func (set KeySet) position(keyId string) int {
	return slices.IndexFunc(set.Keys, func(key JsonWebKey) bool {
		return key.KeyId == keyId && key.Use != set.encryptionUse()
	})
}

func (set KeySet) at(index int) (JsonWebKey, error) {
	if index < 0 {
		return JsonWebKey{}, RefusedToken{Reason: "no signing key has the token's key id"}
	}
	return set.Keys[index], nil
}

func (KeySet) encryptionUse() string {
	return "enc"
}
