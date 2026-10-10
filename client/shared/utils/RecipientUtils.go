import (
	"regexp"
	"slices"
	"strings"
)

// RecipientUtils holds the address-list rules of the composer: splitting a
// comma-separated field, the token being typed, suggestion filtering and the
// automatic CC/BCC merge.
type RecipientUtils struct{}

// Split turns "a, b,,c" into ["a", "b", "c"].
func (RecipientUtils) Split(value string) []string {
	return append([]string{}, RecipientUtils{}.addressPattern().FindAllString(value, -1)...)
}

// Join writes addresses back into one field value.
func (RecipientUtils) Join(addresses []string) string {
	return strings.Join(addresses, ", ")
}

// CurrentToken is the address being typed after the last comma.
func (RecipientUtils) CurrentToken(value string) string {
	return strings.TrimSpace(value[strings.LastIndex(value, RecipientUtils{}.separator())+1:])
}

// ReplaceCurrent puts a chosen address in place of the token being typed.
func (RecipientUtils) ReplaceCurrent(value string, address string) string {
	return RecipientUtils{}.Join(RecipientUtils{}.Split(value[:strings.LastIndex(value, RecipientUtils{}.separator())+1] + address))
}

// Unused keeps at most eight suggestions that are not already a recipient.
func (RecipientUtils) Unused(suggestions []RecipientSuggestion, to string, cc string, blindCopy string) []RecipientSuggestion {
	separator := RecipientUtils{}.separator()
	used := RecipientUtils{}.lowered(RecipientUtils{}.Split(to + separator + cc + separator + blindCopy))
	kept := RecipientUtils{}.notListed(suggestions, used)
	return kept[:min(len(kept), RecipientUtils{}.mostSuggestions())]
}

// Initials gives the two letters of a suggestion avatar.
func (RecipientUtils) Initials(name string, email string) string {
	source := strings.TrimSpace(RecipientUtils{}.firstPresent(name, email))
	parts := RecipientUtils{}.initialSeparators().Split(source, -1)
	return strings.ToUpper(RecipientUtils{}.initialsOf(source, RecipientUtils{}.Split(strings.Join(parts, RecipientUtils{}.separator()))))
}

// MergeAutomatic replaces the previous automatic recipients of a field with the
// next ones, keeping every address the person typed and removing duplicates.
func (RecipientUtils) MergeAutomatic(current string, previous []string, next []string) string {
	kept := RecipientUtils{}.notIn(RecipientUtils{}.Split(current), RecipientUtils{}.lowered(previous))
	return RecipientUtils{}.Join(RecipientUtils{}.unique(RecipientUtils{}.lowered(append(kept, next...))))
}

func (RecipientUtils) addressPattern() *regexp.Regexp {
	return regexp.MustCompile(`[^,\s](?:[^,]*[^,\s])?`)
}

func (RecipientUtils) initialSeparators() *regexp.Regexp {
	return regexp.MustCompile(`[@\s._-]+`)
}

func (RecipientUtils) mostSuggestions() int { return 8 }

func (RecipientUtils) separator() string { return "," }

// Lowered lowercases every address of a list.
func (RecipientUtils) Lowered(values []string) []string {
	return RecipientUtils{}.lowered(values)
}

func (RecipientUtils) lowered(values []string) []string {
	lowered := []string{}
	for _, value := range values {
		lowered = append(lowered, strings.ToLower(value))
	}
	return lowered
}

func (RecipientUtils) notListed(suggestions []RecipientSuggestion, used []string) []RecipientSuggestion {
	kept := []RecipientSuggestion{}
	for _, suggestion := range suggestions {
		kept = append(kept, RecipientUtils{}.unlisted(suggestion, used)...)
	}
	return kept
}

func (RecipientUtils) unlisted(suggestion RecipientSuggestion, used []string) []RecipientSuggestion {
	if slices.Contains(used, strings.ToLower(suggestion.Email)) {
		return nil
	}
	return []RecipientSuggestion{suggestion}
}

func (RecipientUtils) notIn(values []string, removed []string) []string {
	kept := []string{}
	for _, value := range values {
		kept = append(kept, RecipientUtils{}.absent(value, removed)...)
	}
	return kept
}

func (RecipientUtils) absent(value string, removed []string) []string {
	if slices.Contains(removed, strings.ToLower(value)) {
		return nil
	}
	return []string{value}
}

func (RecipientUtils) unique(values []string) []string {
	seen := map[string]bool{}
	kept := []string{}
	for _, value := range values {
		kept = append(kept, RecipientUtils{}.first(value, seen)...)
	}
	return kept
}

func (RecipientUtils) first(value string, seen map[string]bool) []string {
	if seen[value] {
		return nil
	}
	seen[value] = true
	return []string{value}
}

func (RecipientUtils) firstPresent(name string, email string) string {
	if name != "" {
		return name
	} else if email != "" {
		return email
	}
	return "?"
}

func (RecipientUtils) initialsOf(source string, parts []string) string {
	if len(parts) > 1 {
		return string([]rune(parts[0])[:1]) + string([]rune(parts[1])[:1])
	}
	return string([]rune(source)[:min(2, len([]rune(source)))])
}
