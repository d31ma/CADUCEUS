package devicebox

import (
	"encoding/base64"
	"regexp"
	"strings"
)

type Formats struct{}

func (Formats) EnvelopeId(text string) (string, bool) {
	lowered := strings.ToLower(text)
	return lowered, regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`).MatchString(lowered)
}

func (Formats) DeviceId(text string) bool {
	return regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`).MatchString(text)
}

func (formats Formats) DecodedSize(text string) (int, bool) {
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(text, "="))
	return len(decoded), err == nil && len(decoded) > 0
}

func (Formats) Kinds() []string {
	return []string{"message", "group", "linking"}
}

func (Formats) Undelivered() string {
	return "undelivered"
}
