package transport

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
)

type SecretGuard struct {
	secretDigest [sha256.Size]byte
	next         http.Handler
}

func NewSecretGuard(secret string, next http.Handler) SecretGuard {
	return SecretGuard{secretDigest: sha256.Sum256([]byte(secret)), next: next}
}

func (guard SecretGuard) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	guard.admit(writer, request, guard.matches(request.Header.Get(Headers{}.Secret())))
}

func (guard SecretGuard) admit(writer http.ResponseWriter, request *http.Request, matches bool) {
	if !matches {
		http.Error(writer, "unauthorized", http.StatusUnauthorized)
		return
	}
	guard.next.ServeHTTP(writer, request)
}

func (guard SecretGuard) matches(presented string) bool {
	digest := sha256.Sum256([]byte(presented))
	return subtle.ConstantTimeCompare(digest[:], guard.secretDigest[:]) == 1
}
