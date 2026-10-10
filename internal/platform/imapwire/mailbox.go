// Package imapwire contains bounded IMAP wire-format helpers shared by the
// authenticated command boundary and the protocol adapter.
package imapwire

import (
	"encoding/base64"
	"errors"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// EncodeMailbox converts a Unicode mailbox component to IMAP4rev1 modified
// UTF-7. Callers remain responsible for joining hierarchy components with the
// provider-advertised delimiter.
func EncodeMailbox(value string) string {
	var result strings.Builder
	for index := 0; index < len(value); {
		r, size := utf8.DecodeRuneInString(value[index:])
		if r >= 0x20 && r <= 0x7e {
			if r == '&' {
				result.WriteString("&-")
			} else {
				result.WriteRune(r)
			}
			index += size
			continue
		}
		start := index
		for index < len(value) {
			r, size = utf8.DecodeRuneInString(value[index:])
			if r >= 0x20 && r <= 0x7e {
				break
			}
			index += size
		}
		units := utf16.Encode([]rune(value[start:index]))
		bytes := make([]byte, 0, len(units)*2)
		for _, unit := range units {
			bytes = append(bytes, byte(unit>>8), byte(unit))
		}
		encoded := base64.RawStdEncoding.EncodeToString(bytes)
		result.WriteByte('&')
		result.WriteString(strings.ReplaceAll(encoded, "/", ","))
		result.WriteByte('-')
	}
	return result.String()
}

// DecodeMailbox converts an IMAP4rev1 modified UTF-7 mailbox name to Unicode.
func DecodeMailbox(value string) (string, error) {
	var result strings.Builder
	for index := 0; index < len(value); {
		if value[index] != '&' {
			if value[index] > 0x7f {
				return "", errors.New("non-ASCII byte outside modified UTF-7 sequence")
			}
			result.WriteByte(value[index])
			index++
			continue
		}
		end := strings.IndexByte(value[index:], '-')
		if end < 0 {
			return "", errors.New("unterminated modified UTF-7 sequence")
		}
		end += index
		if end == index+1 {
			result.WriteByte('&')
			index = end + 1
			continue
		}
		encoded := strings.ReplaceAll(value[index+1:end], ",", "/")
		decoded, err := base64.RawStdEncoding.DecodeString(encoded)
		if err != nil || len(decoded)%2 != 0 {
			return "", errors.New("invalid modified UTF-7 sequence")
		}
		units := make([]uint16, 0, len(decoded)/2)
		for offset := 0; offset < len(decoded); offset += 2 {
			units = append(units, uint16(decoded[offset])<<8|uint16(decoded[offset+1]))
		}
		result.WriteString(string(utf16.Decode(units)))
		index = end + 1
	}
	return result.String(), nil
}
