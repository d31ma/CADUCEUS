package inbound

import (
	"crypto/sha256"
	"encoding/hex"
)

type Digests struct{}

func (Digests) Hex(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}
