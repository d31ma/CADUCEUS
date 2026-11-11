package mailauth

import (
	"regexp"
	"strings"
)

type rawMessage struct {
	fields []messageField
	body   string
}

func newRawMessage(raw []byte) rawMessage {
	message := rawMessage{}
	text := message.lineEnds(string(raw))
	before, body, _ := strings.Cut(message.newline()+text, message.newline()+message.newline())
	header := strings.TrimPrefix(before, message.newline()) + message.newline()
	return rawMessage{fields: message.split(header), body: body}
}

func (message rawMessage) lineEnds(text string) string {
	return strings.ReplaceAll(strings.ReplaceAll(text, message.newline(), "\n"), "\n", message.newline())
}

func (rawMessage) split(header string) []messageField {
	texts := regexp.MustCompile(`(?m)^[^ \t\r\n][^\r\n]*\r\n(?:[ \t][^\r\n]*\r\n)*`).FindAllString(header, -1)
	fields := make([]messageField, 0, len(texts))
	for _, text := range texts {
		fields = append(fields, newMessageField(text))
	}
	return fields
}

func (rawMessage) newline() string {
	return "\r\n"
}
