import (
	"bytes"
	"encoding/json"
	"math"
	"strconv"
	"strings"
	"unicode/utf16"
)

// TextValue converts decoded JSON values the way the JavaScript server's
// String(), Number(), truthiness, trim() and \s did, so records keep the
// behaviour they had when JavaScript read them.
type TextValue struct{}

// Of answers String(value), with null and undefined as "".
func (text TextValue) Of(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case bool:
		return strconv.FormatBool(typed)
	case float64:
		return text.number(typed)
	case int:
		return strconv.Itoa(typed)
	case []any:
		parts := make([]string, len(typed))
		for index, item := range typed {
			parts[index] = text.Of(item)
		}
		return strings.Join(parts, ",")
	case []string:
		return strings.Join(typed, ",")
	default:
		return "[object Object]"
	}
}

func (TextValue) number(value float64) string {
	if math.IsNaN(value) {
		return "NaN"
	}
	if math.IsInf(value, 0) {
		return map[bool]string{true: "Infinity", false: "-Infinity"}[value > 0]
	}
	if value == math.Trunc(value) && math.Abs(value) < 1e21 {
		return strconv.FormatFloat(value, 'f', -1, 64)
	}
	return strconv.FormatFloat(value, 'g', -1, 64)
}

// Truthy reports whether JavaScript treats value as true.
func (TextValue) Truthy(value any) bool {
	switch typed := value.(type) {
	case nil:
		return false
	case bool:
		return typed
	case string:
		return typed != ""
	case float64:
		return typed != 0 && !math.IsNaN(typed)
	case int:
		return typed != 0
	default:
		return true
	}
}

// Or answers value when it is truthy and fallback otherwise (`value || fallback`).
func (text TextValue) Or(value any, fallback any) any {
	if text.Truthy(value) {
		return value
	}
	return fallback
}

// Number answers `Number(value) || 0`.
func (TextValue) Number(value any) float64 {
	var number float64
	switch typed := value.(type) {
	case float64:
		number = typed
	case int:
		number = float64(typed)
	case bool:
		if typed {
			number = 1
		}
	case string:
		trimmed := TextValue{}.Trim(typed)
		parsed, err := strconv.ParseFloat(trimmed, 64)
		if err == nil {
			number = parsed
		}
	}
	if math.IsNaN(number) {
		return 0
	}
	return number
}

// Space reports whether JavaScript's \s and trim() treat letter as white
// space.
func (TextValue) Space(letter rune) bool {
	switch letter {
	case '\t', '\n', '\v', '\f', '\r', ' ', 0xa0, 0x1680, 0x2028, 0x2029, 0x202f, 0x205f, 0x3000, 0xfeff:
		return true
	}
	return letter >= 0x2000 && letter <= 0x200a
}

// Trim answers value.trim().
func (text TextValue) Trim(value string) string {
	return strings.TrimFunc(value, text.Space)
}

// Words answers value split on runs of white space, without empty parts.
func (text TextValue) Words(value string) []string {
	return strings.FieldsFunc(value, text.Space)
}

// Collapsed answers value with each run of white space replaced by one space.
func (text TextValue) Collapsed(value string) string {
	var collapsed strings.Builder
	spacing := false
	for _, letter := range value {
		if text.Space(letter) {
			spacing = true
			continue
		}
		if spacing {
			collapsed.WriteByte(' ')
			spacing = false
		}
		collapsed.WriteRune(letter)
	}
	if spacing {
		collapsed.WriteByte(' ')
	}
	return collapsed.String()
}

// Length answers value.length: the UTF-16 code units of the text.
func (TextValue) Length(value string) int {
	return len(utf16.Encode([]rune(value)))
}

// Prefix answers value.slice(0, size) in UTF-16 code units.
func (TextValue) Prefix(value string, size int) string {
	units := utf16.Encode([]rune(value))
	if len(units) <= size {
		return value
	}
	return string(utf16.Decode(units[:size]))
}

// Json answers JSON.stringify(value): no HTML escaping, no trailing newline.
func (TextValue) Json(value any) string {
	var encoded bytes.Buffer
	encoder := json.NewEncoder(&encoded)
	encoder.SetEscapeHTML(false)
	if encoder.Encode(value) != nil {
		return ""
	}
	return strings.TrimSuffix(encoded.String(), "\n")
}

// Includes reports whether list holds value as the same text
// (`list.includes(value)`, which never matches a value that is not text).
func (TextValue) Includes(list []string, value any) bool {
	text, isText := value.(string)
	if !isText {
		return false
	}
	for _, item := range list {
		if item == text {
			return true
		}
	}
	return false
}

// List answers value as a list, or nil when it is not one.
func (TextValue) List(value any) []any {
	list, _ := value.([]any)
	return list
}

// Copy answers a shallow copy of fields.
func (TextValue) Copy(fields map[string]any) map[string]any {
	copied := make(map[string]any, len(fields))
	for key, value := range fields {
		copied[key] = value
	}
	return copied
}
