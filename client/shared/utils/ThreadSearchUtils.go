import (
	"regexp"
	"slices"
	"strings"
	"unicode/utf16"
)

// ThreadSearchUtils reads and edits the Mailspring-style search syntax of the
// mailbox search box (from:, to:, in:, before:, since:, after:, is:, has:,
// subject:). A caret position counts UTF-16 units, as the browser reports it.
type ThreadSearchUtils struct{}

// Wrap quotes a search value, dropping the quotes inside it.
func (ThreadSearchUtils) Wrap(value string) string {
	return `"` + strings.TrimSpace(strings.ReplaceAll(value, `"`, "")) + `"`
}

// Current finds the token expression that the caret touches.
func (ThreadSearchUtils) Current(query string, caret int) SearchToken {
	matches := ThreadSearchUtils{}.tokenPattern().FindAllStringSubmatchIndex(query, -1)
	at := ThreadSearchUtils{}.byteOffset(query, caret)
	found := slices.IndexFunc(matches, ThreadSearchUtils{}.covering(at))
	return ThreadSearchUtils{}.tokenAt(query, matches, found)
}

// Segments splits a query into plain text and highlighted field names.
func (ThreadSearchUtils) Segments(query string) []SearchSegment {
	return ThreadSearchUtils{}.segments(query, ThreadSearchUtils{}.fieldPattern().FindAllStringIndex(query, -1))
}

// StaticSuggestions offers field names, or the values of the field under the
// caret, without asking the server.
func (ThreadSearchUtils) StaticSuggestions(query string, caret int) []SearchSuggestion {
	current := ThreadSearchUtils{}.Current(query, caret)
	field, known := ThreadSearchUtils{}.field(current.Token)
	menus := [][]SearchSuggestion{
		ThreadSearchUtils{}.fieldMenu(""),
		{},
		ThreadSearchUtils{}.fieldMenu(current.Token),
		ThreadSearchUtils{}.valueMenu(field.Name, ThreadSearchUtils{}.matchingValues(field, current.Term)),
	}
	return menus[ThreadSearchUtils{}.menuFor(query, current.Token, known)]
}

// Apply puts a chosen suggestion in place of the token under the caret.
func (ThreadSearchUtils) Apply(query string, caret int, suggestion SearchSuggestion) AppliedSearch {
	current := ThreadSearchUtils{}.Current(query, caret)
	index, length := ThreadSearchUtils{}.place(query, current)
	expression := ThreadSearchUtils{}.expression(suggestion)
	next := ThreadSearchUtils{}.joined(query, []int{index, length}, expression)
	return AppliedSearch{Query: ThreadSearchUtils{}.padded(next), Submit: suggestion.Submit}
}

func (ThreadSearchUtils) tokenPattern() *regexp.Regexp {
	return regexp.MustCompile(`(?i)(^|[\s\p{Zs}\x{2028}\x{2029}\x{FEFF}])(i[ns]?|s[iu]?[nb]?[cj]?e?c?t?|fr?o?m?|to?|ha?s?|be?f?o?r?e?|af?t?e?r?)(?::? ?$|: ?("[^"]*"?|[^\s\p{Zs}\x{2028}\x{2029}\x{FEFF}]+))`)
}

func (ThreadSearchUtils) fieldPattern() *regexp.Regexp {
	return regexp.MustCompile(`(?i)\b(from|to|in|before|since|after|is|has|subject):`)
}

func (ThreadSearchUtils) byteOffset(text string, caret int) int {
	units := utf16.Encode([]rune(text))
	return len(string(utf16.Decode(units[:max(0, min(caret, len(units)))])))
}

func (utils ThreadSearchUtils) covering(at int) func([]int) bool {
	return func(match []int) bool {
		start := utils.tokenStart(match)
		if start <= at && match[1] >= at {
			return true
		}
		return false
	}
}

// tokenStart skips the space that the pattern matches before a token.
func (ThreadSearchUtils) tokenStart(match []int) int {
	prefix := match[2:4]
	return match[0] + prefix[1] - prefix[0]
}

func (utils ThreadSearchUtils) tokenAt(query string, matches [][]int, found int) SearchToken {
	if found < 0 {
		return SearchToken{Index: -1}
	}
	match := matches[found]
	start := utils.tokenStart(match)
	token, value := match[4:6], match[6:8]
	term := strings.TrimPrefix(strings.TrimSuffix(query[max(value[0], 0):max(value[1], 0)], `"`), `"`)
	return SearchToken{Token: strings.ToLower(query[token[0]:token[1]]), Term: strings.ToLower(term), Index: start, Length: match[1] - start}
}

func (utils ThreadSearchUtils) segments(query string, matches [][]int) []SearchSegment {
	segments := []SearchSegment{}
	offset := 0
	for _, match := range matches {
		segments = append(segments, utils.textBefore(query, offset, match[0])...)
		segments = append(segments, SearchSegment{Text: query[match[0]:match[1]], Token: true})
		offset = match[1]
	}
	return append(segments, utils.textBefore(query, offset, len(query))...)
}

func (ThreadSearchUtils) textBefore(query string, offset int, end int) []SearchSegment {
	if end <= offset {
		return []SearchSegment{}
	}
	return []SearchSegment{{Text: query[offset:end]}}
}

func (utils ThreadSearchUtils) field(name string) (SearchField, bool) {
	fields := utils.fields()
	found := slices.IndexFunc(fields, func(field SearchField) bool { return field.Name == name })
	if found < 0 {
		return SearchField{}, false
	}
	return fields[found], true
}

func (utils ThreadSearchUtils) fieldMenu(prefix string) []SearchSuggestion {
	menu := []SearchSuggestion{}
	for _, field := range slices.DeleteFunc(utils.fields(), utils.hiddenOrOther(prefix)) {
		menu = append(menu, SearchSuggestion{Token: field.Name, Description: field.Description})
	}
	return menu
}

func (ThreadSearchUtils) hiddenOrOther(prefix string) func(SearchField) bool {
	return func(field SearchField) bool {
		if !field.Shown || !strings.HasPrefix(field.Name, prefix) {
			return true
		}
		return false
	}
}

func (utils ThreadSearchUtils) matchingValues(field SearchField, term string) []string {
	return slices.DeleteFunc(slices.Clone(field.Values), utils.unwantedValue(term))
}

func (ThreadSearchUtils) unwantedValue(term string) func(string) bool {
	return func(value string) bool {
		if term != "" && (!strings.HasPrefix(value, term) || value == term) {
			return true
		}
		return false
	}
}

func (utils ThreadSearchUtils) valueMenu(name string, values []string) []SearchSuggestion {
	menu := []SearchSuggestion{}
	for _, value := range values {
		term := map[bool]string{true: utils.Wrap(value), false: value}[strings.Contains(value, utils.space())]
		menu = append(menu, SearchSuggestion{Token: name, Term: term, Description: value, Submit: true})
	}
	return menu
}

// menuFor picks the field names (0), nothing (1), the field names that start
// with the typed word (2), or the values of a known field (3).
func (ThreadSearchUtils) menuFor(query string, token string, known bool) int {
	if query == "" {
		return 0
	} else if token == "" {
		return 1
	} else if !known {
		return 2
	}
	return 3
}

func (utils ThreadSearchUtils) place(query string, current SearchToken) (int, int) {
	if current.Index == -1 && strings.HasSuffix(query, utils.space()) {
		return len(query), 0
	}
	return current.Index, current.Length
}

func (ThreadSearchUtils) expression(suggestion SearchSuggestion) string {
	if suggestion.Token != "" {
		return suggestion.Token + ":" + suggestion.Term
	}
	return suggestion.Term
}

func (utils ThreadSearchUtils) joined(query string, span []int, expression string) string {
	index, length := span[0], span[1]
	if index == -1 {
		return expression
	}
	parts := []string{strings.TrimSpace(query[:index]), expression, strings.TrimSpace(query[index+length:])}
	return strings.Join(slices.DeleteFunc(parts, func(part string) bool { return part == "" }), utils.space())
}

func (utils ThreadSearchUtils) padded(query string) string {
	if strings.HasSuffix(query, utils.space()) {
		return query
	}
	return query + utils.space()
}

func (ThreadSearchUtils) space() string { return " " }

func (ThreadSearchUtils) fields() []SearchField {
	dates := []string{"yesterday", "2 days ago", "last week", "last month", ">=2018/05/31"}
	received := "date received or range"
	address := "an email address"
	attachment := "attachment"
	return []SearchField{
		{Name: "from", Description: address, Shown: true},
		{Name: "to", Description: address, Shown: true},
		{Name: "in", Description: "folder or label", Shown: true},
		{Name: "before", Description: received, Values: dates, Shown: true},
		{Name: "since", Description: received, Values: dates, Shown: true},
		{Name: "after", Description: received, Values: dates},
		{Name: "is", Description: "unread, read, starred, unstarred", Values: []string{"unread", "read", "starred", "unstarred"}, Shown: true},
		{Name: "has", Description: attachment, Values: []string{attachment}, Shown: true},
		{Name: "subject", Description: "an email subject", Shown: true},
	}
}
