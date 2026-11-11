package compose

import (
	"crypto/rand"
	"encoding/hex"
)

type MessageIds struct{}

func (MessageIds) randomByteCount() int { return 16 }

func (ids MessageIds) New(domain string) (string, error) {
	err := ids.requireDomain(domain)
	if err != nil {
		return "", err
	}
	random := make([]byte, ids.randomByteCount())
	_, _ = rand.Read(random)
	return hex.EncodeToString(random) + "@" + domain, nil
}

func (MessageIds) requireDomain(domain string) error {
	if domain == "" {
		return MissingField{Field: fieldNames{}.Domain()}
	}
	return nil
}
