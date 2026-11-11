package dkim

import (
	"bytes"
	"regexp"
	"strings"
)

type messageReader struct{}

func (messageReader) separator() []byte {
	return []byte("\r\n\r\n")
}

func (messageReader) fieldPattern() *regexp.Regexp {
	return regexp.MustCompile("[!-9;-~]+:(?:[^\r\n]|\r\n[ \t])*")
}

func (reader messageReader) read(raw []byte) (message, error) {
	head, body, err := reader.split(raw)
	if err != nil {
		return message{}, err
	}
	fields, err := reader.fields(head)
	if err != nil {
		return message{}, err
	}
	return message{fields: reader.group(fields), body: body}, nil
}

func (reader messageReader) split(raw []byte) (string, []byte, error) {
	head, body, found := bytes.Cut(raw, reader.separator())
	if !found {
		return "", nil, MalformedMessage{Reason: "there is no empty line after the header"}
	}
	return string(head), body, nil
}

func (reader messageReader) fields(head string) ([]string, error) {
	fields := reader.fieldPattern().FindAllString(head, -1)
	if strings.Join(fields, relaxed{}.lineEnd()) != head {
		return nil, MalformedMessage{Reason: "a header line is not a valid field"}
	}
	return fields, nil
}

func (messageReader) group(fields []string) map[string][]string {
	grouped := map[string][]string{}
	for _, field := range fields {
		name, value, _ := strings.Cut(field, ":")
		grouped[strings.ToLower(name)] = append(grouped[strings.ToLower(name)], value)
	}
	return grouped
}
