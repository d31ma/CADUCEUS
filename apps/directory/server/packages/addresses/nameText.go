package addresses

import (
	"strings"
	"unicode"
)

type nameText struct{}

func (text nameText) lowered(kind string, input string) (string, error) {
	value := strings.TrimSpace(input)
	if strings.ContainsFunc(value, text.notASCII) {
		return "", InvalidName{Kind: kind, Reason: "must hold only ASCII characters"}
	}
	return strings.ToLower(value), nil
}

func (nameText) notASCII(character rune) bool {
	return character > unicode.MaxASCII
}

func (text nameText) splitAtLast(value string) (string, string, bool) {
	index := strings.LastIndex(value, text.at())
	if index < 0 {
		return value, "", false
	}
	return value[:index], value[index+len(text.at()):], true
}

func (nameText) at() string {
	return "@"
}
