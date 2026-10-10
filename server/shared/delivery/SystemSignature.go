import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
)

// SystemSignature checks the X-Caduceus-Signature HMAC that system callers
// (the delivery workers and the inbound receiver) put on their requests.
// The HMAC covers the exact body bytes, or "{}" when there is no body.
type SystemSignature struct{}

// Body answers the request's decoded body once its signature, made with the
// named setting's secret, is valid; or the answer that refuses the request.
func (signature SystemSignature) Body(request *http.Request, setting string) (any, *http.Response) {
	data := []byte{}
	if request.Body != nil {
		data, _ = io.ReadAll(io.LimitReader(request.Body, signature.limit()))
	}
	request.Body = io.NopCloser(bytes.NewReader(data))
	body, refusal := RequestBody{}.Value(request)
	if refusal != nil {
		return nil, refusal
	}
	secret := Settings{}.Value(setting)
	if secret == "" {
		return nil, Answer{}.Json(http.StatusInternalServerError, map[string]any{"error": setting + " is required"})
	}
	if !signature.Valid(data, secret, request.Header.Get("X-Caduceus-Signature")) {
		return nil, Answer{}.Unauthorized("Invalid webhook signature")
	}
	return body, nil
}

// Valid reports whether given is the hex HMAC-SHA256 of the body.
func (SystemSignature) Valid(data []byte, secret string, given string) bool {
	if len(data) == 0 {
		data = []byte("{}")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(data)
	expected := hex.EncodeToString(mac.Sum(nil))
	return len(given) == len(expected) && hmac.Equal([]byte(given), []byte(expected))
}

func (SystemSignature) limit() int64 { return 64 << 20 }
