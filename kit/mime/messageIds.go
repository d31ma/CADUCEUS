package mime

import "strings"

type messageIds struct{}

func (ids messageIds) All(value string) []string {
	spaced := strings.ReplaceAll(charsets{}.Valid(value), ">", "> ")
	return ids.trimmed(strings.Fields(spaced))
}

func (ids messageIds) First(value string) string {
	return ids.first(ids.All(value))
}

func (ids messageIds) trimmed(fields []string) []string {
	var found []string
	for _, field := range fields {
		found = ids.appendId(found, strings.Trim(field, "<>"))
	}
	return found
}

func (messageIds) appendId(found []string, id string) []string {
	if id == "" {
		return found
	}
	return append(found, id)
}

func (messageIds) first(found []string) string {
	if len(found) == 0 {
		return ""
	}
	return found[0]
}
