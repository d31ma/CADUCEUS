import (
	"strconv"
	"strings"
)

// TextUtils holds the small text choices the reader makes in many places.
type TextUtils struct{}

func (TextUtils) Or(value string, fallback string) string {
	return TextUtils{}.or(value, fallback)
}

func (TextUtils) Choose(condition bool, chosen string, otherwise string) string {
	return TextUtils{}.choose(condition, chosen, otherwise)
}

func (TextUtils) Count(count int, one string, many string) string {
	return strconv.Itoa(count) + " " + TextUtils{}.choose(count == 1, one, many)
}

func (TextUtils) NonEmpty(values []string) []string {
	return TextUtils{}.nonEmpty(values)
}

func (TextUtils) nonEmpty(values []string) []string {
	kept := []string{}
	for _, value := range values {
		kept = append(kept, TextUtils{}.present(value)...)
	}
	return kept
}

func (TextUtils) First(values []string) string {
	return strings.Join(values[:min(1, len(values))], "")
}

func (TextUtils) or(value string, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

func (TextUtils) choose(condition bool, chosen string, otherwise string) string {
	if condition {
		return chosen
	}
	return otherwise
}

func (TextUtils) present(value string) []string {
	if value == "" {
		return []string{}
	}
	return []string{value}
}
