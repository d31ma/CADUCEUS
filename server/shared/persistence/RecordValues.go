import "math"

// RecordValues reads the loosely typed fields of decoded records with the
// JavaScript meanings the former repositories relied on: truthiness, typeof
// checks and Number().
type RecordValues struct{}

// Text answers value when it is text, else "".
func (RecordValues) Text(value any) string {
	text, _ := value.(string)
	return text
}

// Object answers value when it is a plain object (not an array or null).
func (RecordValues) Object(value any) (map[string]any, bool) {
	object, isObject := value.(map[string]any)
	return object, isObject && object != nil
}

// List answers value when it is an array.
func (RecordValues) List(value any) ([]any, bool) {
	switch list := value.(type) {
	case []any:
		return list, true
	case []string:
		items := make([]any, len(list))
		for index, item := range list {
			items[index] = item
		}
		return items, true
	}
	return nil, false
}

// Number answers value when it is a number.
func (RecordValues) Number(value any) (float64, bool) {
	switch number := value.(type) {
	case float64:
		return number, true
	case int:
		return float64(number), true
	case int64:
		return float64(number), true
	}
	return 0, false
}

// Truthy reports whether JavaScript treats value as true.
func (values RecordValues) Truthy(value any) bool {
	switch typed := value.(type) {
	case nil:
		return false
	case bool:
		return typed
	case string:
		return typed != ""
	}
	if number, isNumber := values.Number(value); isNumber {
		return number != 0 && !math.IsNaN(number)
	}
	return true
}

// Or answers value when it is truthy, else fallback (JavaScript ||).
func (values RecordValues) Or(value any, fallback any) any {
	if values.Truthy(value) {
		return value
	}
	return fallback
}

// Coalesce answers value unless it is null or missing (JavaScript ??).
func (RecordValues) Coalesce(value any, fallback any) any {
	if value == nil {
		return fallback
	}
	return value
}

// NumberOf answers Number(value) || 0 for a stored counter.
func (values RecordValues) NumberOf(value any) float64 {
	number, isNumber := values.Number(value)
	if !isNumber || math.IsNaN(number) {
		return 0
	}
	return number
}
