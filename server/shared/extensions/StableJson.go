import (
	"fmt"
	"sort"
	"strings"
)

// StableJson writes a value as the JavaScript stableJSON helper did: object
// keys sorted, arrays in order, and every scalar exactly as JSON.stringify
// wrote it, so digests computed by either server match.
type StableJson struct{}

// Text answers the stable JSON text of value.
func (stable StableJson) Text(value any) string {
	switch typed := value.(type) {
	case []any:
		parts := make([]string, len(typed))
		for index, item := range typed {
			parts[index] = stable.Text(item)
		}
		return "[" + strings.Join(parts, ",") + "]"
	case []string:
		parts := make([]string, len(typed))
		for index, item := range typed {
			parts[index] = stable.Quote(item)
		}
		return "[" + strings.Join(parts, ",") + "]"
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		parts := make([]string, len(keys))
		for index, key := range keys {
			parts[index] = stable.Quote(key) + ":" + stable.Text(typed[key])
		}
		return "{" + strings.Join(parts, ",") + "}"
	case string:
		return stable.Quote(typed)
	case nil:
		return "null"
	case bool, float64, int:
		return TextValue{}.Of(typed)
	}
	return "null"
}

// Quote answers JSON.stringify(text): only the quote, the backslash and
// control characters are escaped.
func (StableJson) Quote(text string) string {
	var quoted strings.Builder
	quoted.WriteByte('"')
	for _, character := range text {
		switch character {
		case '"':
			quoted.WriteString(`\"`)
		case '\\':
			quoted.WriteString(`\\`)
		case '\b':
			quoted.WriteString(`\b`)
		case '\f':
			quoted.WriteString(`\f`)
		case '\n':
			quoted.WriteString(`\n`)
		case '\r':
			quoted.WriteString(`\r`)
		case '\t':
			quoted.WriteString(`\t`)
		default:
			if character < 0x20 {
				quoted.WriteString(fmt.Sprintf(`\u%04x`, character))
			} else {
				quoted.WriteRune(character)
			}
		}
	}
	quoted.WriteByte('"')
	return quoted.String()
}
