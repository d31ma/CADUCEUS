import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
)

// ResultAnswer answers what the Go owner returned, unchanged, with the status
// the former route layer derived from an answer's shape: an object whose
// "error" is text takes the status of its first marker (unauthorized,
// forbidden, notFound ...), or 400 when "error" is its only field.
type ResultAnswer struct{}

// Of answers the owner's result; a missing result is an empty 204 answer.
func (answer ResultAnswer) Of(result json.RawMessage) *http.Response {
	if len(bytes.TrimSpace(result)) == 0 {
		return &http.Response{StatusCode: http.StatusNoContent, Header: http.Header{}, Body: http.NoBody}
	}
	header := http.Header{"Content-Type": {"application/json; charset=utf-8"}}
	return &http.Response{StatusCode: answer.status(result), Header: header, Body: io.NopCloser(bytes.NewReader(result)), ContentLength: int64(len(result))}
}

// Under answers {field: result}; a missing result leaves the field out.
func (answer ResultAnswer) Under(field string, result json.RawMessage) *http.Response {
	wrapped := map[string]json.RawMessage{}
	if len(bytes.TrimSpace(result)) > 0 {
		wrapped[field] = result
	}
	return Answer{}.Ok(wrapped)
}

func (answer ResultAnswer) status(result json.RawMessage) int {
	var fields map[string]json.RawMessage
	if json.Unmarshal(result, &fields) != nil || fields == nil {
		return http.StatusOK
	}
	if !bytes.HasPrefix(bytes.TrimSpace(fields["error"]), []byte(`"`)) {
		return http.StatusOK
	}
	for _, marker := range answer.markers() {
		if _, present := fields[marker.field]; present {
			return marker.status
		}
	}
	if len(fields) == 1 {
		return http.StatusBadRequest
	}
	return http.StatusOK
}

func (ResultAnswer) markers() []struct {
	field  string
	status int
} {
	return []struct {
		field  string
		status int
	}{
		{"unauthorized", http.StatusUnauthorized},
		{"forbidden", http.StatusForbidden},
		{"notFound", http.StatusNotFound},
		{"conflict", http.StatusConflict},
		{"blocked", http.StatusUnprocessableEntity},
		{"rateLimited", http.StatusTooManyRequests},
		{"unavailable", http.StatusServiceUnavailable},
	}
}
