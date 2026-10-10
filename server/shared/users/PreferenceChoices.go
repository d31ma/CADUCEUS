import "sort"

// PreferenceChoices picks stored preference values from fixed catalogs,
// falling back to the default for anything else (a missing field, another
// type or an unknown identifier).
type PreferenceChoices struct{}

// Text answers value when it is one of the allowed identifiers, else fallback.
func (PreferenceChoices) Text(value any, allowed []string, fallback string) string {
	text, isText := value.(string)
	if !isText {
		return fallback
	}
	for _, candidate := range allowed {
		if candidate == text {
			return text
		}
	}
	return fallback
}

// Number answers value when it is one of the allowed numbers, else fallback.
func (PreferenceChoices) Number(value any, allowed []float64, fallback float64) float64 {
	number, isNumber := RecordValues{}.Number(value)
	if !isNumber {
		return fallback
	}
	for _, candidate := range allowed {
		if candidate == number {
			return number
		}
	}
	return fallback
}

// Boolean answers value when it is a boolean, else fallback.
func (PreferenceChoices) Boolean(value any, fallback bool) bool {
	flag, isFlag := value.(bool)
	if !isFlag {
		return fallback
	}
	return flag
}

// Object answers value when it is a plain object, else an empty one.
func (PreferenceChoices) Object(value any) map[string]any {
	object, isObject := RecordValues{}.Object(value)
	if !isObject {
		return map[string]any{}
	}
	return object
}

// Domains answers the distinct valid domains of a list, in order.
func (PreferenceChoices) Domains(value any) []string {
	list, _ := RecordValues{}.List(value)
	domains := []string{}
	seen := map[string]bool{}
	for _, item := range list {
		text, isText := item.(string)
		domain := Security{}.NormalizeDomain(text)
		if isText && domain != "" && !seen[domain] {
			seen[domain] = true
			domains = append(domains, domain)
		}
	}
	return domains
}

// Includes reports whether list holds value.
func (PreferenceChoices) Includes(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

// Keys answers the keys of object in a stable order: stored JSON objects come
// back without the order they were written in.
func (PreferenceChoices) Keys(object map[string]any) []string {
	keys := []string{}
	for key := range object {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// Addresses answers the distinct valid email addresses of a list, in order.
func (PreferenceChoices) Addresses(list []any) []string {
	addresses := []string{}
	for _, item := range list {
		text, _ := item.(string)
		address := Security{}.NormalizeEmailAddress(text)
		if address != "" && !(PreferenceChoices{}).Includes(addresses, address) {
			addresses = append(addresses, address)
		}
	}
	return addresses
}
