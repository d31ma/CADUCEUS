import (
	"encoding/json"
	"slices"
	"strings"
)

// PreferenceUtils reads untrusted JSON values the way the JavaScript client
// did: a value of the wrong shape becomes the zero value of the wanted type.
type PreferenceUtils struct{}

// Decode copies value into target through JSON. Fields that value does not
// hold keep the values target already had, so callers prefill defaults.
func (PreferenceUtils) Decode(value any, target any) {
	text, _ := json.Marshal(value)
	_ = json.Unmarshal(text, target)
}

func (PreferenceUtils) Object(value any) map[string]any {
	object, _ := value.(map[string]any)
	return object
}

func (PreferenceUtils) Text(values map[string]any, key string) string {
	text, _ := values[key].(string)
	return text
}

func (PreferenceUtils) Flag(values map[string]any, key string) bool {
	flag, _ := values[key].(bool)
	return flag
}

func (PreferenceUtils) Strings(values map[string]any, key string) []string {
	list := []string{}
	PreferenceUtils{}.Decode(values[key], &list)
	return list
}

// Choice keeps value when allowed lists it and otherwise answers fallback.
func (PreferenceUtils) Choice(value string, allowed []string, fallback string) string {
	return PreferenceUtils{}.pick(value, allowed, fallback)
}

func (PreferenceUtils) Lower(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

// Unique answers the distinct, non-empty, lower-case members of values in order.
func (PreferenceUtils) Unique(values []string) []string {
	seen := map[string]bool{}
	return slices.DeleteFunc(PreferenceUtils{}.lowered(values), func(value string) bool {
		repeated := seen[value]
		seen[value] = true
		return repeated
	})
}

// Joined writes an address list the way its text field shows it.
func (PreferenceUtils) Joined(values []string) string {
	return strings.Join(values, ", ")
}

func (PreferenceUtils) Without(values []string, removed string) []string {
	return slices.DeleteFunc(slices.Clone(values), func(value string) bool { return value == removed })
}

func (PreferenceUtils) Toggle(values []string, value string) []string {
	return PreferenceUtils{}.toggled(values, value)
}

func (PreferenceUtils) pick(value string, allowed []string, fallback string) string {
	if slices.Contains(allowed, value) {
		return value
	}
	return fallback
}

func (PreferenceUtils) lowered(values []string) []string {
	lowered := []string{}
	for _, value := range values {
		lowered = append(lowered, strings.ToLower(strings.TrimSpace(value)))
	}
	return slices.DeleteFunc(lowered, func(value string) bool { return value == "" })
}

func (PreferenceUtils) toggled(values []string, value string) []string {
	if slices.Contains(values, value) {
		return PreferenceUtils{}.Without(values, value)
	}
	return append(slices.Clone(values), value)
}
