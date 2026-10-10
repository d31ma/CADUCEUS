import (
	"encoding/json"
	"errors"
	"net/http"
	"time"
)

// ProviderAccounts carries the external-account operations the calendar and
// contacts routes need (CalDAV resources, CardDAV contacts and their
// mutations) to the Go owner, which holds every account and credential.
// It is the calendar and contacts slice of the former ExternalAccounts
// facade; the external accounts routes keep their own.
type ProviderAccounts struct{}

// Call runs one accounts.* operation for the caller's bearer token and
// answers its decoded result.
func (accounts ProviderAccounts) Call(operation string, parameters map[string]any) (any, error) {
	payload := map[string]any{"op": operation, "params": parameters}
	frame, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	if len(frame) > accounts.requestLimit() {
		return nil, &BoundaryError{Code: "EBADREQUEST", Message: "Boundary request is too large"}
	}
	raw, err := Boundary{}.FromSettings().Call(payload, accounts.responseLimit(), accounts.timeout())
	if err != nil {
		return nil, err
	}
	var result any
	if len(raw) == 0 {
		return nil, nil
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, &BoundaryError{Code: "EUNAVAILABLE", Message: "The Go boundary sent malformed JSON"}
	}
	return result, nil
}

// Object runs one operation and answers its result as an object (empty when
// it is not one).
func (accounts ProviderAccounts) Object(operation string, parameters map[string]any) (map[string]any, error) {
	result, err := accounts.Call(operation, parameters)
	object, _ := RecordValues{}.Object(result)
	if object == nil {
		object = map[string]any{}
	}
	return object, err
}

// Failure answers a failed account operation with the status of its stable
// boundary code; anything else reads as the service being unavailable.
func (ProviderAccounts) Failure(err error) *http.Response {
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
	}
	return Answer{}.Unavailable(failure.Message)
}

// IsBoundary reports whether err came from the boundary (the former
// AuthBridgeError).
func (ProviderAccounts) IsBoundary(err error) bool {
	var failure *BoundaryError
	return errors.As(err, &failure)
}

// requestLimit keeps a request below the owner's 64 MiB frame ceiling once
// the signed envelope encodes it again.
func (ProviderAccounts) requestLimit() int { return 47 << 20 }

func (ProviderAccounts) responseLimit() int { return 64 << 20 }

func (ProviderAccounts) timeout() time.Duration { return 15 * time.Second }
