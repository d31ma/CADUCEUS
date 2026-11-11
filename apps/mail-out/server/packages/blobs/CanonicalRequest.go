package blobs

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

type CanonicalRequest struct {
	Method        string
	Path          string
	Query         string
	Headers       string
	SignedHeaders string
	PayloadHash   string
}

func (request CanonicalRequest) Hash() string {
	text := strings.Join([]string{request.Method, request.Path, request.Query, request.Headers, request.SignedHeaders, request.PayloadHash}, "\n")
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}
