package dkim

import "strings"

type message struct {
	fields map[string][]string
	body   []byte
}

func (parsed message) count(name string) int {
	return len(parsed.fields[name])
}

func (parsed message) fromText() string {
	return strings.ReplaceAll(strings.Join(parsed.fields[headerNames{}.from()], ","), relaxed{}.lineEnd(), "")
}

func (parsed message) take(name string) string {
	values := parsed.fields[name]
	if len(values) == 0 {
		return ""
	}
	parsed.fields[name] = values[:len(values)-1]
	return relaxed{}.header(name, values[len(values)-1]) + relaxed{}.lineEnd()
}
