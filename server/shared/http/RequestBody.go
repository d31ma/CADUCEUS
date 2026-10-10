import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// RequestBody reads a route's JSON body.
type RequestBody struct{}

// Object answers the body as a JSON object (nil when the body is empty or not
// an object), or the 400 answer earlier versions gave for malformed JSON.
func (RequestBody) Object(request *http.Request) (map[string]any, *http.Response) {
	value, refusal := RequestBody{}.Value(request)
	object, _ := value.(map[string]any)
	return object, refusal
}

// Value answers the decoded body: nil when empty, the text itself when it is
// not JSON, or the 400 answer for a JSON body that does not parse.
func (RequestBody) Value(request *http.Request) (any, *http.Response) {
	if request.Body == nil {
		return nil, nil
	}
	data, err := io.ReadAll(io.LimitReader(request.Body, RequestBody{}.limit()))
	if err != nil || len(data) == 0 {
		return nil, nil
	}
	contentType := request.Header.Get("Content-Type")
	if contentType != "" && !strings.Contains(strings.ToLower(contentType), "json") {
		return string(data), nil
	}
	var value any
	if json.Unmarshal(data, &value) != nil {
		return nil, Answer{}.BadRequest("The request body is not valid JSON")
	}
	return value, nil
}

func (RequestBody) limit() int64 { return 64 << 20 }
