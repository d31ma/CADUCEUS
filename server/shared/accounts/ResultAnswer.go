import (
	"bytes"
	"encoding/json"
	"net/http"
)

// ResultAnswer answers what the Go owner returned, unchanged, with the status
// the former route layer derived from an answer's shape: an object whose
// "error" is text takes the status of its first marker (unauthorized,
// forbidden, notFound ...), or 400 when "error" is its only field.
type ResultAnswer struct{}

// Of answers the owner's result; a missing result is an empty 204 answer.
func (ResultAnswer) Of(result json.RawMessage) *http.Response {
	return RawAnswer{}.Of(result)
}

// Under answers {field: result}; a missing result leaves the field out.
func (answer ResultAnswer) Under(field string, result json.RawMessage) *http.Response {
	wrapped := map[string]json.RawMessage{}
	if len(bytes.TrimSpace(result)) > 0 {
		wrapped[field] = result
	}
	return Answer{}.Ok(wrapped)
}
