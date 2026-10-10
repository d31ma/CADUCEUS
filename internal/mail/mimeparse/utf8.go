package mimeparse

import (
	"strings"
	"unicode/utf8"
)

// NodeUTF8 decodes data as Node's Buffer.toString('utf8') does: each maximal
// invalid subpart (WHATWG Encoding) becomes one U+FFFD.
func NodeUTF8(data []byte) string {
	if utf8.Valid(data) {
		return string(data)
	}
	var builder strings.Builder
	builder.Grow(len(data) + 16)
	for index := 0; index < len(data); {
		r, size := utf8.DecodeRune(data[index:])
		if r != utf8.RuneError || size > 1 {
			builder.WriteRune(r)
			index += size
			continue
		}
		index += invalidSubpart(data[index:])
		builder.WriteRune(utf8.RuneError)
	}
	return builder.String()
}

func invalidSubpart(data []byte) int {
	lead := data[0]
	var need int
	low, high := byte(0x80), byte(0xbf)
	switch {
	case lead >= 0xc2 && lead <= 0xdf:
		need = 1
	case lead >= 0xe0 && lead <= 0xef:
		need = 2
		if lead == 0xe0 {
			low = 0xa0
		} else if lead == 0xed {
			high = 0x9f
		}
	case lead >= 0xf0 && lead <= 0xf4:
		need = 3
		if lead == 0xf0 {
			low = 0x90
		} else if lead == 0xf4 {
			high = 0x8f
		}
	default:
		return 1
	}
	consumed := 1
	for consumed <= need && consumed < len(data) {
		next := data[consumed]
		if next < low || next > high {
			break
		}
		low, high = 0x80, 0xbf
		consumed++
	}
	return consumed
}
