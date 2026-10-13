package accountids

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"

	"github.com/d31ma/caduceus/internal/accounts"
)

type Generator struct {
	random io.Reader
}

func New() *Generator {
	return &Generator{random: rand.Reader}
}

func (generator *Generator) NewAccountID() (accounts.AccountID, error) {
	bytes := make([]byte, 18)
	if _, err := io.ReadFull(generator.random, bytes); err != nil {
		return "", fmt.Errorf("generate external account identifier: %w", err)
	}
	return accounts.AccountID("ext_" + base64.RawURLEncoding.EncodeToString(bytes)), nil
}
