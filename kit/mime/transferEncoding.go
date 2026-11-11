package mime

import (
	"bytes"
	"encoding/base64"
	"io"
	"mime/quotedprintable"
	"strings"
)

type transferEncoding struct{}

func (encoding transferEncoding) Decode(name string, content []byte) []byte {
	decode, known := encoding.decoders()[strings.ToLower(strings.TrimSpace(name))]
	return encoding.pick(decode, known)(content)
}

func (encoding transferEncoding) decoders() map[string]func([]byte) []byte {
	return map[string]func([]byte) []byte{
		"base64":           encoding.base64,
		"quoted-printable": encoding.quotedPrintable,
	}
}

func (encoding transferEncoding) pick(decode func([]byte) []byte, known bool) func([]byte) []byte {
	if known {
		return decode
	}
	return encoding.unchanged
}

func (transferEncoding) unchanged(content []byte) []byte {
	return content
}

func (encoding transferEncoding) base64(content []byte) []byte {
	letters := encoding.base64Letters(content)
	decoded := make([]byte, base64.RawStdEncoding.DecodedLen(len(letters)))
	count, _ := base64.RawStdEncoding.Decode(decoded, letters)
	return decoded[:count]
}

func (encoding transferEncoding) base64Letters(content []byte) []byte {
	letters := make([]byte, 0, len(content))
	for _, value := range content {
		letters = encoding.appendLetter(letters, value)
	}
	return letters
}

func (transferEncoding) appendLetter(letters []byte, value byte) []byte {
	if strings.IndexByte(base64Alphabet{}.Letters(), value) < 0 {
		return letters
	}
	return append(letters, value)
}

func (transferEncoding) quotedPrintable(content []byte) []byte {
	decoded, _ := io.ReadAll(quotedprintable.NewReader(bytes.NewReader(content)))
	return decoded
}
