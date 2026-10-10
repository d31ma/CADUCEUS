import (
	"encoding/base64"
	"strings"
)

// Base64Text decodes base64 as Node's Buffer.from(text, 'base64') does:
// standard and URL alphabets alike, other characters skipped, decoding
// ending at the first padding character.
type Base64Text struct{}

// Decoded answers the bytes text encodes.
func (Base64Text) Decoded(text string) []byte {
	if padding := strings.IndexByte(text, '='); padding >= 0 {
		text = text[:padding]
	}
	var clean strings.Builder
	for _, letter := range text {
		switch {
		case letter == '-':
			clean.WriteByte('+')
		case letter == '_':
			clean.WriteByte('/')
		case letter == '+' || letter == '/' || (letter >= 'A' && letter <= 'Z') || (letter >= 'a' && letter <= 'z') || (letter >= '0' && letter <= '9'):
			clean.WriteRune(letter)
		}
	}
	encoded := clean.String()
	if len(encoded)%4 == 1 {
		encoded = encoded[:len(encoded)-1]
	}
	decoded, _ := base64.RawStdEncoding.DecodeString(encoded)
	return decoded
}
