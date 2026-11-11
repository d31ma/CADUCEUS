package dkim

import (
	"slices"
	"strings"
)

type headerNames struct{}

func (headerNames) from() string {
	return "from"
}

func (names headerNames) defaults() []string {
	return []string{
		names.from(), "to", "cc", "reply-to", "subject", "date", "message-id", "in-reply-to",
		"references", "mime-version", "content-type", "content-transfer-encoding",
	}
}

func (names headerNames) candidates(requested []string) []string {
	if len(requested) == 0 {
		return names.defaults()
	}
	return strings.Split(strings.ToLower(strings.Join(requested, ":")), ":")
}

func (names headerNames) signed(parsed message, candidates []string) []string {
	signed := []string{}
	for _, name := range candidates {
		signed = append(signed, slices.Repeat([]string{name}, parsed.count(name))...)
	}
	return append(signed, names.from())
}

func (headerNames) canonical(parsed message, signed []string) string {
	var builder strings.Builder
	for _, name := range signed {
		builder.WriteString(parsed.take(name))
	}
	return builder.String()
}
