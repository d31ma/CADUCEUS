package mailauth

import "strings"

type messageField struct {
	name string
	raw  string
}

func newMessageField(raw string) messageField {
	name, _, _ := strings.Cut(raw, ":")
	return messageField{name: strings.TrimSpace(name), raw: raw}
}

func (field messageField) value() string {
	_, value, _ := strings.Cut(field.raw, ":")
	return value
}

func (field messageField) named(name string) bool {
	return strings.EqualFold(field.name, name)
}
