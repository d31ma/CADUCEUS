import (
	"bytes"
	"io"
	"strings"
	"unicode/utf8"
)

// MimeCharset turns the bytes of a declared charset into UTF-8 text with the
// standard library alone: UTF-8 and US-ASCII as they are, ISO-8859-1 and
// Windows-1252 by their code charts. Any other charset is read as UTF-8 with
// invalid bytes replaced.
type MimeCharset struct{}

// Text answers content in the named charset as UTF-8.
func (charset MimeCharset) Text(name string, content []byte) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "iso-8859-1", "latin1", "latin-1", "iso8859-1", "l1", "cp819":
		return charset.chart(content, nil)
	case "windows-1252", "cp1252", "x-cp1252":
		return charset.chart(content, charset.windowsHigh())
	}
	if utf8.Valid(content) {
		return string(content)
	}
	return strings.ToValidUTF8(string(content), "�")
}

// Reader answers input decoded into UTF-8, for encoded header words.
func (charset MimeCharset) Reader(name string, input io.Reader) (io.Reader, error) {
	content, err := io.ReadAll(input)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader([]byte(charset.Text(name, content))), nil
}

func (MimeCharset) chart(content []byte, high []rune) string {
	var text strings.Builder
	for _, value := range content {
		if high != nil && value >= 0x80 && value < 0xa0 {
			text.WriteRune(high[value-0x80])
			continue
		}
		text.WriteRune(rune(value))
	}
	return text.String()
}

// windowsHigh answers the Windows-1252 characters of bytes 0x80 to 0x9f.
func (MimeCharset) windowsHigh() []rune {
	return []rune{
		0x20ac, 0x81, 0x201a, 0x0192, 0x201e, 0x2026, 0x2020, 0x2021, 0x02c6, 0x2030, 0x0160, 0x2039, 0x0152, 0x8d, 0x017d, 0x8f,
		0x90, 0x2018, 0x2019, 0x201c, 0x201d, 0x2022, 0x2013, 0x2014, 0x02dc, 0x2122, 0x0161, 0x203a, 0x0153, 0x9d, 0x017e, 0x0178,
	}
}
