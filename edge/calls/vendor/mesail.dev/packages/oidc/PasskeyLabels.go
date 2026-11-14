package oidc

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

type PasskeyLabels struct{}

func (labels PasskeyLabels) Unique(wanted string, existing []PasskeyRecord) string {
	return labels.free(strings.TrimSpace(wanted), labels.taken(existing))
}

func (labels PasskeyLabels) Checked(label string) error {
	length := utf8.RuneCountInString(strings.TrimSpace(label))
	return labels.lengthAllowed(length)
}

func (labels PasskeyLabels) free(base string, taken map[string]bool) string {
	label := base
	for number := 2; taken[label]; number++ {
		label = labels.numbered(base, number)
	}
	return label
}

func (labels PasskeyLabels) numbered(base string, number int) string {
	suffix := " " + strconv.Itoa(number)
	return labels.cut(base, labels.longest()-len(suffix)) + suffix
}

func (PasskeyLabels) cut(text string, most int) string {
	characters := []rune(text)
	if len(characters) <= most {
		return text
	}
	return strings.TrimSpace(string(characters[:most]))
}

func (PasskeyLabels) taken(existing []PasskeyRecord) map[string]bool {
	taken := map[string]bool{}
	for _, record := range existing {
		taken[record.Label] = true
	}
	return taken
}

func (labels PasskeyLabels) lengthAllowed(length int) error {
	if length < 1 || length > labels.longest() {
		return InvalidLabel{}
	}
	return nil
}

func (PasskeyLabels) longest() int {
	return 64
}
