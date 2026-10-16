import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"time"
)

// IdentityUtils makes the identifiers that the client sends with a change,
// such as a mailbox action's clientRequestId or a folder mutation's
// idempotency key. They need to be unique, not secret. The SecureRandom
// browser API crashed the TinyGo module when called from an event listener,
// and crypto/rand crashed it at start, so the identifier is a digest of the
// change and the current time.
type IdentityUtils struct{}

// UUID is a version-4-shaped identifier derived from seed and the time.
func (IdentityUtils) UUID(seed string) string {
	digest := sha256.Sum256([]byte(seed + "|" + strconv.FormatInt(time.Now().UnixNano(), 36)))
	bytes := digest[:IdentityUtils{}.size()]
	bytes[IdentityUtils{}.versionByte()] = bytes[IdentityUtils{}.versionByte()]&0x0f | 0x40
	bytes[IdentityUtils{}.variantByte()] = bytes[IdentityUtils{}.variantByte()]&0x3f | 0x80
	return IdentityUtils{}.grouped(bytes)
}

// grouped writes the bytes as hexadecimal groups of 4, 2, 2, 2 and 6 bytes.
func (IdentityUtils) grouped(bytes []byte) string {
	groups := []string{}
	for _, length := range []int{4, 2, 2, 2, 6} {
		groups, bytes = append(groups, hex.EncodeToString(bytes[:length])), bytes[length:]
	}
	return strings.Join(groups, "-")
}

func (IdentityUtils) size() int { return 16 }

func (IdentityUtils) versionByte() int { return 6 }

func (IdentityUtils) variantByte() int { return 8 }
