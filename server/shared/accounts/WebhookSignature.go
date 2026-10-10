import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// WebhookSignature checks the x-caduceus-signature header of an internal
// workload request: the hex HMAC-SHA256, under a deployment secret, of the
// JSON body ("{}" when there is none).
//
// The former route signed JSON.stringify of the decoded body. It signs the
// body the sender wrote instead (compacted when it is JSON), which is the
// same text for every JSON sender and also holds when the sender sends the
// JSON as text/plain, where re-encoding the decoded text quoted it and
// refused every correctly signed request.
type WebhookSignature struct{}

// Refusal answers nil for a correctly signed request, the 400 answer for a
// malformed JSON body, or the 401 answer.
func (signature WebhookSignature) Refusal(request *http.Request, settingName string) *http.Response {
	body, err := signature.body(request)
	if err != nil {
		return Answer{}.BadRequest("The request body is not valid JSON")
	}
	secret := Settings{}.Value(settingName)
	if secret == "" {
		return Answer{}.Json(http.StatusInternalServerError, map[string]any{"error": settingName + " is required"})
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	given := request.Header.Get("x-caduceus-signature")
	if len(given) != len(expected) || !hmac.Equal([]byte(strings.ToLower(given)), []byte(expected)) {
		return Answer{}.Unauthorized("Invalid webhook signature")
	}
	return nil
}

// body answers the signed text: "{}" for an empty body, the compacted body
// when it is JSON, else the body as it came. A body declared as JSON (or
// undeclared) that does not parse is an error.
func (WebhookSignature) body(request *http.Request) ([]byte, error) {
	if request.Body == nil {
		return []byte("{}"), nil
	}
	data, err := io.ReadAll(io.LimitReader(request.Body, 64<<20))
	if err != nil || len(data) == 0 {
		return []byte("{}"), nil
	}
	var compact bytes.Buffer
	if json.Compact(&compact, data) == nil {
		return compact.Bytes(), nil
	}
	contentType := request.Header.Get("Content-Type")
	if contentType == "" || strings.Contains(strings.ToLower(contentType), "json") {
		return nil, json.Unmarshal(data, new(any))
	}
	return data, nil
}
