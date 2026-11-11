package textindex

import (
	"slices"
	"strings"
	"unicode"
)

type Tokenizer struct{}

func (tokenizer Tokenizer) Terms(text string) []string {
	words := strings.FieldsFunc(strings.ToLower(text), tokenizer.isSeparator)
	slices.Sort(words)
	return slices.Compact(words)
}

func (Tokenizer) isSeparator(character rune) bool {
	return !unicode.In(character, unicode.Letter, unicode.Digit)
}
