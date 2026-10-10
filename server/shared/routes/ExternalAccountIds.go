import (
	"encoding/json"
	"errors"
	"net/http"
	"time"
)

// ExternalAccountIds lists the ids of the caller's connected accounts
// through the external account boundary. It is the one accounts.list call
// the preference route needs; the external accounts routes own the rest.
type ExternalAccountIds struct{}

// Of answers the account ids the token's caller has connected, or the answer
// that reports the boundary's failure.
func (accounts ExternalAccountIds) Of(token string) ([]string, *http.Response) {
	request := map[string]any{"op": "accounts.list", "params": map[string]any{"token": token}}
	raw, err := Boundary{}.FromSettings().Call(request, accounts.responseLimit(), accounts.timeout())
	if err != nil {
		return nil, accounts.failure(err)
	}
	var listed any
	_ = json.Unmarshal(raw, &listed)
	entries, isList := RecordValues{}.List(listed)
	if !isList {
		object, _ := RecordValues{}.Object(listed)
		entries, _ = RecordValues{}.List(object["accounts"])
	}
	ids := []string{}
	for _, entry := range entries {
		account, _ := RecordValues{}.Object(entry)
		if id, isText := account["id"].(string); isText && id != "" {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// A listing can be large; the frame stays below the boundary's 64 MiB limit.
func (ExternalAccountIds) responseLimit() int { return 64 << 20 }

func (ExternalAccountIds) timeout() time.Duration { return 15 * time.Second }

// failure answers a stable boundary failure with its own message.
func (ExternalAccountIds) failure(err error) *http.Response {
	var failure *BoundaryError
	if !errors.As(err, &failure) {
		return Answer{}.Unavailable("External account service unavailable")
	}
	switch failure.Code {
	case "EBADREQUEST", "EUNSUPPORTED":
		return Answer{}.BadRequest(failure.Message)
	case "EUNAUTHORIZED":
		return Answer{}.Unauthorized(failure.Message)
	case "EFORBIDDEN", "EPOLICY":
		return Answer{}.Forbidden(failure.Message)
	case "ENOTFOUND":
		return Answer{}.NotFound(failure.Message)
	case "ECONFLICT":
		return Answer{}.Conflict(failure.Message)
	case "EAUTHREQUIRED":
		return Answer{}.Blocked(failure.Message, []string{"credentials"})
	default:
		return Answer{}.Unavailable(failure.Message)
	}
}
