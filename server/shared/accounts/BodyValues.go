import (
	"math"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf16"
)

// BodyValues reads a decoded request body with the JavaScript meanings the
// former account routes relied on: optional chaining (body?.field), object
// spread ({...body}), String(value || ''), Number() and
// Number.isSafeInteger.
type BodyValues struct{}

// Field answers body?.[name]: the field of an object body, else nil.
func (BodyValues) Field(body any, name string) any {
	object, _ := body.(map[string]any)
	return object[name]
}

// Has answers name in body for an object body.
func (BodyValues) Has(body any, name string) bool {
	object, _ := body.(map[string]any)
	_, present := object[name]
	return present
}

// Spread answers {...(body || {})}: the fields of an object, the indexed
// items of an array or the indexed UTF-16 units of a text.
func (values BodyValues) Spread(body any) map[string]any {
	spread := map[string]any{}
	switch typed := body.(type) {
	case map[string]any:
		for name, value := range typed {
			spread[name] = value
		}
	case []any:
		for index, value := range typed {
			spread[strconv.Itoa(index)] = value
		}
	case string:
		for index, unit := range utf16.Encode([]rune(typed)) {
			spread[strconv.Itoa(index)] = string(utf16.Decode([]uint16{unit}))
		}
	}
	return spread
}

// Text answers String(value || '').
func (BodyValues) Text(value any) string {
	return RecordValues{}.String(RecordValues{}.Or(value, ""))
}

// Trimmed answers String(value || '').trim().
func (values BodyValues) Trimmed(value any) string {
	return strings.TrimFunc(values.Text(value), values.space)
}

// TrimmedText answers value.trim() when value is text, else "".
func (values BodyValues) TrimmedText(value any) string {
	text, _ := value.(string)
	return strings.TrimFunc(text, values.space)
}

// space answers whether JavaScript's trim removes the character: Unicode
// white space, line terminators and the byte order mark.
func (BodyValues) space(character rune) bool {
	return unicode.IsSpace(character) || character == 0xFEFF
}

// PositiveRevision answers value when it is a safe positive integer number
// (Number.isSafeInteger(value) && value > 0), else 0.
func (values BodyValues) PositiveRevision(value any) int64 {
	number, isNumber := RecordValues{}.Number(value)
	if !isNumber || !values.safeInteger(number) || number <= 0 {
		return 0
	}
	return int64(number)
}

// CoercedRevision answers Number(value) when it is a safe integer of at
// least 1, else 0.
func (values BodyValues) CoercedRevision(value any) int64 {
	number := RecordValues{}.Numeric(value)
	if !values.safeInteger(number) || number < 1 {
		return 0
	}
	return int64(number)
}

func (BodyValues) safeInteger(number float64) bool {
	return !math.IsNaN(number) && !math.IsInf(number, 0) && number == math.Trunc(number) && math.Abs(number) <= 1<<53-1
}

// Limit answers Math.max(1, Math.min(500, Number(text) || 50)) for a query
// value, which may be missing.
func (BodyValues) Limit(text string, present bool) float64 {
	number := math.NaN()
	if present {
		number = RecordValues{}.Numeric(text)
	}
	if math.IsNaN(number) || number == 0 {
		number = 50
	}
	return math.Max(1, math.Min(500, number))
}
