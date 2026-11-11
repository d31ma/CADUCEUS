package connections

import (
	"crypto/rand"
	"encoding/base64"
)

type IdSource struct{}

func (source IdSource) Next() string {
	random := make([]byte, source.size())
	_, _ = rand.Read(random)
	return base64.RawURLEncoding.EncodeToString(random)
}

func (IdSource) size() int { return 16 }
