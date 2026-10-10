import (
	"net/http"
	"strings"
)

// BodyText reads request values the way the former routes did with
// String(value ?? '').trim().
type BodyText struct{}

// Of answers one body field as trimmed text: "" when it is missing or null.
func (text BodyText) Of(body map[string]any, field string) string {
	return Security{}.Trim(text.String(body[field]))
}

// String answers String(value ?? ''): numbers and booleans as JavaScript
// prints them, and a list as its items joined with commas.
func (text BodyText) String(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case []any:
		items := make([]string, len(typed))
		for index, item := range typed {
			items[index] = text.String(item)
		}
		return strings.Join(items, ",")
	}
	return RecordValues{}.String(value)
}

// Address answers the caller's network address as the proxy reported it:
// the first X-Forwarded-For entry, else X-Real-IP, else "".
func (BodyText) Address(request *http.Request) string {
	forwarded := strings.TrimSpace(strings.Split(request.Header.Get("X-Forwarded-For"), ",")[0])
	if forwarded != "" {
		return forwarded
	}
	return request.Header.Get("X-Real-IP")
}
