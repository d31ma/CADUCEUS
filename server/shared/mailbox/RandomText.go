import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// RandomText makes the random identifiers, digests and timestamps the
// mailbox records carry, in the JavaScript server's encodings.
type RandomText struct{}

// Hex answers size random bytes as lower-case hex (Security.randomText).
func (RandomText) Hex(size int) string {
	value := make([]byte, size)
	_, _ = rand.Read(value)
	return hex.EncodeToString(value)
}

// Digest answers the SHA-256 digest of value as lower-case hex.
func (RandomText) Digest(value []byte) string {
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}

// Now answers the current time as new Date().toISOString() wrote it.
func (RandomText) Now() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
}

// Millis answers Date.now().
func (RandomText) Millis() int64 {
	return time.Now().UnixMilli()
}
