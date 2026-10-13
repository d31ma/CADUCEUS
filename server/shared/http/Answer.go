import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
)

// Answer builds the HTTP answers CADUCEUS routes give. Errors keep the
// shapes earlier CADUCEUS versions answered: a message under "error" and one
// marker per status, such as {"error": "...", "notFound": true}.
type Answer struct{}

// Json answers value as JSON with a status.
func (Answer) Json(status int, value any) *http.Response {
	body, err := json.Marshal(value)
	if err != nil {
		status, body = http.StatusInternalServerError, []byte(`{"error":"Internal server error"}`)
	}
	header := http.Header{"Content-Type": {"application/json; charset=utf-8"}}
	return &http.Response{StatusCode: status, Header: header, Body: io.NopCloser(bytes.NewReader(body)), ContentLength: int64(len(body))}
}

func (answer Answer) BadRequest(message string) *http.Response {
	return answer.Json(http.StatusBadRequest, map[string]any{"error": message})
}

func (answer Answer) Unauthorized(message string) *http.Response {
	return answer.Json(http.StatusUnauthorized, map[string]any{"error": message, "unauthorized": true})
}

func (answer Answer) Forbidden(message string) *http.Response {
	return answer.Json(http.StatusForbidden, map[string]any{"error": message, "forbidden": true})
}

func (answer Answer) NotFound(message string) *http.Response {
	return answer.Json(http.StatusNotFound, map[string]any{"error": message, "notFound": true})
}

func (answer Answer) Conflict(message string) *http.Response {
	return answer.Json(http.StatusConflict, map[string]any{"error": message, "conflict": true})
}

func (answer Answer) Blocked(message string, blocked []string) *http.Response {
	return answer.Json(http.StatusUnprocessableEntity, map[string]any{"error": message, "blocked": blocked})
}

func (answer Answer) RateLimited(message string, retryAfterSeconds int) *http.Response {
	return answer.Json(http.StatusTooManyRequests, map[string]any{"error": message, "retryAfterSeconds": retryAfterSeconds, "rateLimited": true})
}

func (answer Answer) Unavailable(message string) *http.Response {
	return answer.Json(http.StatusServiceUnavailable, map[string]any{"error": message, "unavailable": true})
}

// Ok answers value with status 200.
func (answer Answer) Ok(value any) *http.Response {
	return answer.Json(http.StatusOK, value)
}
