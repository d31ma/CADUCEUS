import (
	"log"
	"io"
	"net/http"
	"strings"
)

// DeliveryReply turns what a delivery route answers into the HTTP response the
// former RouteRequest.respond built: a response stays as it is, nothing is a
// 204, and a JSON value takes its status from its error markers.
type DeliveryReply struct{}

// Of answers the response for a route's answer.
func (reply DeliveryReply) Of(answer any) *http.Response {
	switch typed := answer.(type) {
	case *http.Response:
		return typed
	case nil:
		return &http.Response{StatusCode: http.StatusNoContent, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(""))}
	}
	return Answer{}.Json(reply.Status(answer), answer)
}

// Status answers the status of an answer: 200 unless it carries an error
// text, then the status of its first marker, else 400 when the error is its
// only field.
func (reply DeliveryReply) Status(answer any) int {
	record, isRecord := answer.(map[string]any)
	if !isRecord {
		return http.StatusOK
	}
	if _, isText := record["error"].(string); !isText {
		return http.StatusOK
	}
	for _, marker := range reply.markers() {
		if _, marked := record[marker.field]; marked {
			return marker.status
		}
	}
	if len(record) == 1 {
		return http.StatusBadRequest
	}
	return http.StatusOK
}

func (DeliveryReply) markers() []struct {
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

// Settled answers the response for a route's answer, or the 500 a thrown
// error became when err is set.
func (reply DeliveryReply) Settled(answer any, err error) *http.Response {
	if err != nil {
		log.Printf("[delivery] request failed: %v", err)
		return Answer{}.Json(http.StatusInternalServerError, map[string]any{"error": "Internal server error"})
	}
	return reply.Of(answer)
}

// Caller answers a signed-in request's JSON body and caller, reading the
// body first as the former RouteRequest did (malformed JSON is a 400 before
// authorization), or the answer that refuses the request.
func (DeliveryReply) Caller(access *AccessHelper, request *http.Request) (RouteCaller, map[string]any, *http.Response) {
	body, refusal := RequestBody{}.Object(request)
	if refusal != nil {
		return RouteCaller{}, nil, refusal
	}
	caller, refusal := access.Signed(request)
	if body == nil {
		body = map[string]any{}
	}
	return caller, body, refusal
}
