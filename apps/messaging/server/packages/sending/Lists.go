package sending

import "strings"

type Lists struct{}

func (Lists) copied(items []string) []string {
	return append([]string{}, items...)
}

func (lists Lists) last(items []string, count int) []string {
	return lists.copied(items[max(0, len(items)-count):])
}

func (Lists) oneIf(condition bool) int {
	if condition {
		return 1
	}
	return 0
}

func (Lists) address(text string) string {
	return strings.ToLower(strings.TrimSpace(text))
}
