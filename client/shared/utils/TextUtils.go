import (
	"slices"
	"strconv"
	"strings"
)

// TextUtils holds the small text choices that several services make.
type TextUtils struct{}

// FirstOf answers the preferred text, or the fallback when it is empty.
func (TextUtils) FirstOf(preferred string, fallback string) string {
	return TextUtils{}.either(preferred, fallback)
}

func (TextUtils) either(preferred string, fallback string) string {
	if preferred != "" {
		return preferred
	}
	return fallback
}

// Choose answers the chosen text when the condition holds, or the other one.
func (TextUtils) Choose(condition bool, chosen string, otherwise string) string {
	return TextUtils{}.choose(condition, chosen, otherwise)
}

func (TextUtils) choose(condition bool, chosen string, otherwise string) string {
	if condition {
		return chosen
	}
	return otherwise
}

// Count is "1 message" or "3 messages".
func (TextUtils) Count(count int, one string, many string) string {
	return strconv.Itoa(count) + " " + TextUtils{}.Choose(count == 1, one, many)
}

// NonEmpty keeps the texts that are not empty.
func (TextUtils) NonEmpty(values []string) []string {
	return slices.DeleteFunc(slices.Clone(values), func(value string) bool { return value == "" })
}

// First is the first text, or "" for none.
func (TextUtils) First(values []string) string {
	return strings.Join(values[:min(1, len(values))], "")
}
