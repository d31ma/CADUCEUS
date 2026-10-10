import (
	"encoding/json"
	"errors"
	"net/http"
	"time"
)

// ExternalAccounts carries one external account operation (accounts.list,
// accounts.create ...) to the Go owner, which runs the provider protocols,
// OAuth, discovery and the account journal. Nothing here decides anything:
// it forwards the caller's bearer token with the operation and answers what
// the owner answered.
type ExternalAccounts struct {
	Boundary Boundary
}

// FromSettings answers the accounts transport the deployment configures.
func (ExternalAccounts) FromSettings() ExternalAccounts {
	return ExternalAccounts{Boundary: Boundary{}.FromSettings()}
}

// Call runs one accounts operation. A 25 MiB MIME source expands once in
// rawRFC822Base64 and once in the signed frame, so requests stay below the
// owner's 64 MiB frame ceiling while bounded sources still fit in answers.
func (accounts ExternalAccounts) Call(operation string, parameters map[string]any) (json.RawMessage, error) {
	payload := map[string]any{"op": operation, "params": parameters}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	if len(encoded) > accounts.requestLimit() {
		return nil, &BoundaryError{Code: "EBADREQUEST", Message: "Boundary request is too large"}
	}
	return accounts.Boundary.Call(json.RawMessage(encoded), accounts.responseLimit(), accounts.timeout())
}

func (ExternalAccounts) requestLimit() int { return 47 << 20 }

func (ExternalAccounts) responseLimit() int { return 64 << 20 }

func (ExternalAccounts) timeout() time.Duration { return 15 * time.Second }

// Failure converts only the owner's stable failures into route answers;
// anything else is the account service being unavailable.
func (ExternalAccounts) Failure(err error) *http.Response {
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
