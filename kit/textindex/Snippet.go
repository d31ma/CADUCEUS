package textindex

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

type Snippet struct {
	Width int
}

func (snippet Snippet) Around(text string, queryTerms []string) string {
	runes := []rune(text)
	start := snippet.start(snippet.firstHit(text, queryTerms), len(runes))
	end := min(len(runes), start+snippet.Width)
	return snippet.before(start) + strings.TrimSpace(string(runes[start:end])) + snippet.after(end, len(runes))
}

func (Snippet) firstHit(text string, queryTerms []string) int {
	lowered := strings.Map(unicode.ToLower, text)
	for _, term := range queryTerms {
		if index := strings.Index(lowered, term); index >= 0 {
			return utf8.RuneCountInString(lowered[:index])
		}
	}
	return 0
}

func (snippet Snippet) start(hit int, length int) int {
	wanted := hit - snippet.Width/4
	return max(0, min(wanted, length-snippet.Width))
}

func (Snippet) before(start int) string {
	if start > 0 {
		return Snippet{}.marker()
	}
	return ""
}

func (Snippet) after(end int, length int) string {
	if end < length {
		return Snippet{}.marker()
	}
	return ""
}

func (Snippet) marker() string {
	return "..."
}
