import (
	"encoding/json"
	"errors"
	"net/http"
	"time"
)

// ProviderAccounts carries the mailbox operations of connected (external)
// accounts to the Go owner, which reauthorizes every call by the caller's
// bearer token. The mailbox routes need only the message, folder and
// mutation operations of the external account boundary.
type ProviderAccounts struct {
	Boundary Boundary
}

// FromSettings answers the accounts boundary the deployment configures.
func (ProviderAccounts) FromSettings() ProviderAccounts {
	return ProviderAccounts{Boundary: Boundary{}.FromSettings()}
}

// A 25 MiB MIME source expands once in rawRFC822Base64 and once in the
// signed bridge envelope; requests stay below the owner's 64 MiB frame.
func (ProviderAccounts) requestLimit() int { return 47 << 20 }

func (ProviderAccounts) responseLimit() int { return 64 << 20 }

func (ProviderAccounts) timeout() time.Duration { return 15 * time.Second }

// Call runs one accounts operation and decodes its result into an object.
func (accounts ProviderAccounts) Call(operation string, parameters map[string]any) (map[string]any, error) {
	payload := map[string]any{"op": operation, "params": parameters}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	if len(encoded) > accounts.requestLimit() {
		return nil, &BoundaryError{Code: "EBADREQUEST", Message: "Boundary request is too large"}
	}
	raw, err := accounts.Boundary.Call(payload, accounts.responseLimit(), accounts.timeout())
	if err != nil {
		return nil, err
	}
	result := map[string]any{}
	_ = json.Unmarshal(raw, &result)
	return result, nil
}

// Status answers the HTTP status of a boundary failure code.
func (ProviderAccounts) Status(err error) int {
	var failure *BoundaryError
	if !errors.As(err, &failure) {
		return 0
	}
	statuses := map[string]int{
		"EBADREQUEST": 400, "EUNSUPPORTED": 400, "EUNAUTHORIZED": 401, "EFORBIDDEN": 403, "EPOLICY": 403,
		"ENOTFOUND": 404, "ECONFLICT": 409, "EAUTHREQUIRED": 422,
	}
	if status, known := statuses[failure.Code]; known {
		return status
	}
	return 503
}

// Failure answers a stable boundary failure with its route status; anything
// else is the service being unavailable.
func (accounts ProviderAccounts) Failure(err error) *http.Response {
	answer := Answer{}
	var failure *BoundaryError
	if !errors.As(err, &failure) {
		return answer.Unavailable("External account service unavailable")
	}
	switch accounts.Status(err) {
	case 400:
		return answer.BadRequest(failure.Message)
	case 401:
		return answer.Unauthorized(failure.Message)
	case 403:
		return answer.Forbidden(failure.Message)
	case 404:
		return answer.NotFound(failure.Message)
	case 409:
		return answer.Conflict(failure.Message)
	case 422:
		return answer.Blocked(failure.Message, []string{"credentials"})
	}
	return answer.Unavailable(failure.Message)
}

// Get answers one connected account of the caller.
func (accounts ProviderAccounts) Get(token string, id string) (map[string]any, error) {
	return accounts.Call("accounts.get", map[string]any{"token": token, "id": id})
}

// Folders answers the folders of one connected account.
func (accounts ProviderAccounts) Folders(token string, id string) (map[string]any, error) {
	return accounts.Call("accounts.folders", map[string]any{"token": token, "id": id})
}

// UnifiedMessages answers the caller's provider message index rows.
func (accounts ProviderAccounts) UnifiedMessages(token string, input map[string]any) (map[string]any, error) {
	parameters := TextValue{}.Copy(input)
	parameters["token"] = token
	return accounts.Call("accounts.unified_messages", parameters)
}

// MessageSource answers one provider message with its RFC 822 source.
func (accounts ProviderAccounts) MessageSource(token string, id string, stableId string) (map[string]any, error) {
	return accounts.Call("accounts.message_source", map[string]any{"token": token, "id": id, "stableId": stableId})
}

// Message answers one provider message's metadata without its source.
func (accounts ProviderAccounts) Message(token string, id string, stableId string) (map[string]any, error) {
	return accounts.Call("accounts.message", map[string]any{"token": token, "id": id, "stableId": stableId})
}

// Mutate queues one provider mutation.
func (accounts ProviderAccounts) Mutate(token string, id string, input map[string]any) (map[string]any, error) {
	parameters := TextValue{}.Copy(input)
	parameters["token"] = token
	parameters["id"] = id
	return accounts.Call("accounts.mutate", parameters)
}

// Mutation answers the state of one queued provider mutation.
func (accounts ProviderAccounts) Mutation(token string, id string, mutationId string) (map[string]any, error) {
	return accounts.Call("accounts.mutation", map[string]any{"token": token, "id": id, "mutationId": mutationId})
}
