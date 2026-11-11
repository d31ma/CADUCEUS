package compose

import (
	"encoding/base64"
	"mime"
	"mime/quotedprintable"
	"net/mail"
	"strings"
)

type encoder struct{}

func (encoder) quotedPrintable(text string) string {
	var buffer strings.Builder
	writer := quotedprintable.NewWriter(&buffer)
	_, _ = writer.Write([]byte(text))
	_ = writer.Close()
	return buffer.String()
}

func (encoder) base64Lines(content []byte) string {
	encoded := base64.StdEncoding.EncodeToString(content)
	width := limits{}.Base64LineLength()
	lines := []string{}
	for start := 0; start < len(encoded); start += width {
		lines = append(lines, encoded[start:min(start+width, len(encoded))])
	}
	return strings.Join(lines, limits{}.LineEnd())
}

func (encoder) words(text string) string {
	return mime.QEncoding.Encode("utf-8", text)
}

func (encoder) address(address Address) string {
	return (&mail.Address{Name: address.Name, Address: address.Email}).String()
}

func (encoder encoder) addressList(addresses []Address) string {
	formatted := make([]string, 0, len(addresses))
	for _, address := range addresses {
		formatted = append(formatted, encoder.address(address))
	}
	return strings.Join(formatted, ", ")
}

func (encoder) fileName(name string) string {
	return mime.FormatMediaType("attachment", map[string]string{"filename": name})
}
