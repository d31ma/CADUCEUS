import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
)

// RawAnswer answers a JSON value a boundary returned, as the former route
// layer answered whatever a route returned: no value is 204, and an error
// shape takes the status of its marker.
type RawAnswer struct{}

// Of answers raw with the status its shape implies.
func (answer RawAnswer) Of(raw json.RawMessage) *http.Response {
	if len(bytes.TrimSpace(raw)) == 0 {
		return &http.Response{StatusCode: http.StatusNoContent, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(nil))}
	}
	var value any
	_ = json.Unmarshal(raw, &value)
	header := http.Header{"Content-Type": {"application/json; charset=utf-8"}}
	return &http.Response{StatusCode: answer.Status(value), Header: header, Body: io.NopCloser(bytes.NewReader(raw)), ContentLength: int64(len(raw))}
}

// Status answers the status the former route layer gave a returned value:
// 200, unless it is an error shape.
func (answer RawAnswer) Status(value any) int {
	record, isObject := value.(map[string]any)
	if !isObject {
		return http.StatusOK
	}
	if _, isText := record["error"].(string); !isText {
		return http.StatusOK
	}
	for _, marker := range answer.markers() {
		if _, present := record[marker.Field]; present {
			return marker.Status
		}
	}
	if len(record) == 1 {
		return http.StatusBadRequest
	}
	return http.StatusOK
}

func (RawAnswer) markers() []StatusMarker {
	return []StatusMarker{
		{Field: "unauthorized", Status: http.StatusUnauthorized},
		{Field: "forbidden", Status: http.StatusForbidden},
		{Field: "notFound", Status: http.StatusNotFound},
		{Field: "conflict", Status: http.StatusConflict},
		{Field: "blocked", Status: http.StatusUnprocessableEntity},
		{Field: "rateLimited", Status: http.StatusTooManyRequests},
		{Field: "unavailable", Status: http.StatusServiceUnavailable},
	}
}
