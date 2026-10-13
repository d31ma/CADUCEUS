import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"regexp"
)

// CapabilityToken mints and reads the bearer capabilities CADUCEUS hands out
// once and stores only as a SHA-256 hash: "<prefix><base64url domain>.<43
// base64url characters>". Thread shares use no prefix, MCP access "mcp1.".
// The domain names the tenant storage to look the hash up in; it is checked
// to be the canonical encoding of a valid domain.
type CapabilityToken struct {
	Prefix string
}

// Create answers a new token for the domain.
func (token CapabilityToken) Create(domain string) string {
	return token.Prefix + base64.RawURLEncoding.EncodeToString([]byte(domain)) + "." + token.Random(32)
}

// Random answers size random bytes as unpadded base64url.
func (CapabilityToken) Random(size int) string {
	secret := make([]byte, size)
	_, _ = rand.Read(secret)
	return base64.RawURLEncoding.EncodeToString(secret)
}

// Parse answers the domain and hash of a well-formed token, or false.
func (token CapabilityToken) Parse(value string, maximumLength int) (ParsedCapability, bool) {
	match := regexp.MustCompile(`^` + regexp.QuoteMeta(token.Prefix) + `([A-Za-z0-9_-]{4,340})\.([A-Za-z0-9_-]{43})$`).FindStringSubmatch(value)
	if match == nil || len(value) > maximumLength {
		return ParsedCapability{}, false
	}
	decoded, err := base64.RawURLEncoding.DecodeString(match[1])
	if err != nil {
		return ParsedCapability{}, false
	}
	domain := Security{}.NormalizeDomain(string(decoded))
	if domain == "" || base64.RawURLEncoding.EncodeToString([]byte(domain)) != match[1] {
		return ParsedCapability{}, false
	}
	return ParsedCapability{Domain: domain, Hash: token.Hash(value)}, true
}

// Hash answers the SHA-256 digest of a token as hex.
func (CapabilityToken) Hash(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

// EqualHash compares two hex digests in constant time.
func (CapabilityToken) EqualHash(left string, right string) bool {
	pattern := regexp.MustCompile(`^[a-f0-9]{64}$`)
	if !pattern.MatchString(left) || !pattern.MatchString(right) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}
