import (
	"crypto/rand"
	"encoding/hex"
	"strings"
)

// IdentityUtils makes the random identifiers that the client sends with a
// change, such as a mailbox action's clientRequestId or an idempotency key.
// It reads crypto/rand rather than the SecureRandom browser API, which
// crashed the TinyGo module when called from an event listener.
type IdentityUtils struct{}

// UUID is a random version 4 UUID.
func (IdentityUtils) UUID() string {
	bytes := make([]byte, IdentityUtils{}.size())
	rand.Read(bytes)
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
