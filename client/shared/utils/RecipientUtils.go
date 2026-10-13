import (
	"regexp"
	"slices"
	"strconv"
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
func (RecipientUtils) Unused(suggestions []ContactSuggestion, to string, cc string, blindCopy string) []ContactSuggestion {
	separator := RecipientUtils{}.separator()
	used := RecipientUtils{}.lowered(RecipientUtils{}.Split(to + separator + cc + separator + blindCopy))
	kept := RecipientUtils{}.notListed(suggestions, used)
	return kept[:min(len(kept), RecipientUtils{}.mostSuggestions())]
}

// initials gives the two letters of a suggestion avatar.
func (RecipientUtils) initials(name string, email string) string {
	source := strings.TrimSpace(RecipientUtils{}.firstPresent(name, email))
	parts := RecipientUtils{}.initialSeparators().Split(source, -1)
	return strings.ToUpper(RecipientUtils{}.initialsOf(source, RecipientUtils{}.Split(strings.Join(parts, RecipientUtils{}.separator()))))
}

// Options are the rows of the suggestion list under a recipient field.
func (RecipientUtils) Options(suggestions []ContactSuggestion, field string, active int) []RecipientOption {
	return RecipientUtils{}.options(suggestions, field, active)
}

func (RecipientUtils) options(suggestions []ContactSuggestion, field string, active int) []RecipientOption {
	options := []RecipientOption{}
	for index, suggestion := range suggestions {
		options = append(options, RecipientOption{
			Index: index, OptionID: "compose-" + field + "-suggestions-" + strconv.Itoa(index), Label: TextUtils{}.FirstOf(suggestion.Name, suggestion.Email),
			Detail: RecipientUtils{}.detail(suggestion), Initials: RecipientUtils{}.initials(suggestion.Name, suggestion.Email), Active: index == active,
		})
	}
	return options
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

func (RecipientUtils) notListed(suggestions []ContactSuggestion, used []string) []ContactSuggestion {
	kept := []ContactSuggestion{}
	for _, suggestion := range suggestions {
		kept = append(kept, RecipientUtils{}.unlisted(suggestion, used)...)
	}
	return kept
}

func (RecipientUtils) unlisted(suggestion ContactSuggestion, used []string) []ContactSuggestion {
	if slices.Contains(used, strings.ToLower(suggestion.Email)) {
		return nil
	}
	return []ContactSuggestion{suggestion}
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

func (RecipientUtils) detail(suggestion ContactSuggestion) string {
	named := map[bool]string{true: suggestion.Email}[suggestion.Name != ""]
	return TextUtils{}.FirstOf(named, map[bool]string{true: "Found in mail", false: "Contact"}[suggestion.Source == "mail"])
}
