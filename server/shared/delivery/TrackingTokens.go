import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// TrackingTokens creates and verifies the authenticated, encrypted tracking
// capabilities that tracked mail carries: "v1.<iv>.<ciphertext>.<tag>", each
// part base64url, AES-256-GCM under a key derived from TRACKING_SECRET (or
// INBOUND_WEBHOOK_SECRET). The public endpoints trust no tracking id,
// recipient or redirect target outside a token. Secret, when set, replaces
// the configured secret.
type TrackingTokens struct {
	Secret string
}

// Created answers the token of a capability, issued at now (milliseconds).
func (tokens TrackingTokens) Created(capability TrackingCapability, now int64) (string, error) {
	payload := map[string]any{
		"v": tokens.version(),
		"t": tokens.shortType(capability.Type),
		"i": tokens.normalizedTrackingId(capability.TrackingId),
		"e": now + tokens.lifetime(),
	}
	if recipient := tokens.normalizedRecipient(capability.Recipient); recipient != "" {
		payload["r"] = recipient
	}
	if capability.Type == tokens.click() {
		payload["u"] = tokens.NormalizedUrl(capability.Url)
		payload["n"] = tokens.normalizedLinkIndex(float64(capability.LinkIndex))
		if payload["u"] == "" || payload["n"].(int) < 0 {
			return "", errors.New("Invalid tracking capability")
		}
	}
	if payload["i"] == "" {
		return "", errors.New("Invalid tracking capability")
	}
	sealer, err := tokens.cipher()
	if err != nil {
		return "", err
	}
	nonce := make([]byte, tokens.nonceBytes())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := sealer.Seal(nil, nonce, []byte(StoredRecords{}.JsonText(payload)), []byte(tokens.associatedData()))
	split := len(sealed) - sealer.Overhead()
	encoding := base64.RawURLEncoding
	return strings.Join([]string{"v" + strconv.Itoa(tokens.version()), encoding.EncodeToString(nonce), encoding.EncodeToString(sealed[:split]), encoding.EncodeToString(sealed[split:])}, "."), nil
}

// Read answers the capability of a valid, unexpired token of the expected
// type ("open" or "click"; "" accepts either), checked at now. Invalid,
// expired, oversized or type-confused values answer false without saying
// why.
func (tokens TrackingTokens) Read(value string, expectedType string, now int64) (TrackingCapability, bool) {
	none := TrackingCapability{}
	token := Security{}.Trim(value)
	if token == "" || len(token) > tokens.longestToken() {
		return none, false
	}
	decoded, readable := tokens.opened(token)
	if !readable {
		return none, false
	}
	capability := TrackingCapability{
		Type:       tokens.longType(decoded["t"]),
		TrackingId: tokens.normalizedTrackingId(decoded["i"]),
		Recipient:  tokens.normalizedRecipient(decoded["r"]),
	}
	expiresAt, isNumber := decoded["e"].(float64)
	if decoded["v"] != float64(tokens.version()) || capability.Type == "" || capability.TrackingId == "" || !isNumber || !tokens.safeInteger(expiresAt) {
		return none, false
	}
	capability.ExpiresAt = int64(expiresAt)
	if capability.ExpiresAt < now || (expectedType != "" && capability.Type != expectedType) {
		return none, false
	}
	if capability.Type != tokens.click() {
		return capability, true
	}
	capability.Url = tokens.NormalizedUrl(decoded["u"])
	index := RecordValues{}.Numeric(decoded["n"])
	if capability.Url == "" || !tokens.safeInteger(index) || index < 0 || index >= float64(tokens.mostLinks()) {
		return none, false
	}
	capability.LinkIndex = int(index)
	return capability, true
}

// opened decrypts a token's payload; the parts must be canonical base64url.
func (tokens TrackingTokens) opened(token string) (map[string]any, bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 4 || parts[0] != "v"+strconv.Itoa(tokens.version()) {
		return nil, false
	}
	encoding := base64.RawURLEncoding
	decodedParts := [][]byte{}
	for _, part := range parts[1:] {
		bytes, err := encoding.DecodeString(part)
		if part == "" || err != nil || encoding.EncodeToString(bytes) != part {
			return nil, false
		}
		decodedParts = append(decodedParts, bytes)
	}
	nonce, ciphertext, tag := decodedParts[0], decodedParts[1], decodedParts[2]
	if len(nonce) != tokens.nonceBytes() || len(tag) != tokens.tagBytes() || len(ciphertext) == 0 {
		return nil, false
	}
	opener, err := tokens.cipher()
	if err != nil {
		return nil, false
	}
	plain, err := opener.Open(nil, nonce, append(append([]byte{}, ciphertext...), tag...), []byte(tokens.associatedData()))
	if err != nil {
		return nil, false
	}
	decoded := map[string]any{}
	if json.Unmarshal(plain, &decoded) != nil {
		return nil, false
	}
	return decoded, true
}

// NormalizedUrl answers an absolute credential-free http(s) URL as a
// browser's URL parser writes it, or "".
func (tokens TrackingTokens) NormalizedUrl(value any) string {
	source, _ := value.(string)
	source = Security{}.Trim(source)
	if source == "" || len(source) > tokens.longestUrl() || regexp.MustCompile(`[\x00-\x1f\x7f]`).MatchString(source) {
		return ""
	}
	parsed, err := url.Parse(source)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil || parsed.Host == "" || parsed.Opaque != "" {
		return ""
	}
	parsed.Host = strings.ToLower(parsed.Host)
	if (parsed.Scheme == "http" && parsed.Port() == "80") || (parsed.Scheme == "https" && parsed.Port() == "443") {
		parsed.Host = parsed.Hostname()
	}
	if parsed.Path == "" {
		parsed.Path = "/"
	}
	return parsed.String()
}

func (tokens TrackingTokens) cipher() (cipher.AEAD, error) {
	secret := tokens.Secret
	settings := Settings{}
	if secret == "" {
		secret = settings.Value("TRACKING_SECRET")
	}
	if secret == "" {
		secret = settings.Value("INBOUND_WEBHOOK_SECRET")
	}
	if len(secret) < tokens.shortestSecret() {
		return nil, errors.New("TRACKING_SECRET or INBOUND_WEBHOOK_SECRET must contain at least 16 characters")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte("caduceus-tracking-aead-v1"))
	block, err := aes.NewCipher(mac.Sum(nil))
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func (TrackingTokens) safeInteger(value float64) bool {
	return value == math.Trunc(value) && math.Abs(value) <= 9007199254740991
}

func (tokens TrackingTokens) shortType(kind string) string {
	if kind == tokens.click() {
		return "c"
	}
	return "o"
}

func (tokens TrackingTokens) longType(value any) string {
	switch value {
	case "o":
		return tokens.open()
	case "c":
		return tokens.click()
	}
	return ""
}

func (TrackingTokens) normalizedTrackingId(value any) string {
	return TrackingRecords{}.NormalizeTrackingId(value)
}

func (tokens TrackingTokens) normalizedRecipient(value any) string {
	text, _ := value.(string)
	recipient := strings.ToLower(Security{}.Trim(text))
	if len(recipient) > tokens.longestRecipient() || !regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`).MatchString(recipient) {
		return ""
	}
	return recipient
}

func (tokens TrackingTokens) normalizedLinkIndex(index float64) int {
	if tokens.safeInteger(index) && index >= 0 && index < float64(tokens.mostLinks()) {
		return int(index)
	}
	return -1
}

func (TrackingTokens) open() string { return "open" }

func (TrackingTokens) click() string { return "click" }

func (TrackingTokens) version() int { return 1 }

func (TrackingTokens) lifetime() int64 { return 2 * 365 * 24 * 60 * 60 * 1000 }

func (TrackingTokens) longestToken() int { return 8192 }

func (TrackingTokens) longestUrl() int { return 2048 }

func (TrackingTokens) longestRecipient() int { return 320 }

func (TrackingTokens) mostLinks() int { return 100 }

func (TrackingTokens) nonceBytes() int { return 12 }

func (TrackingTokens) tagBytes() int { return 16 }

func (TrackingTokens) shortestSecret() int { return 16 }

func (TrackingTokens) associatedData() string { return "caduceus-tracking-v1" }
