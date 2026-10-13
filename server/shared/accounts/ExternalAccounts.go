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
// the owner answered. Every area that reaches connected accounts (the
// accounts routes, mail reading, delivery, calendars and contacts) uses
// this one client. Token is the caller's bearer token for the operations
// that act for one caller (For, OnAccount, List).
type ExternalAccounts struct {
	Boundary Boundary
	Token    string
}

// FromSettings answers the accounts transport the deployment configures.
func (ExternalAccounts) FromSettings() ExternalAccounts {
	return ExternalAccounts{Boundary: Boundary{}.FromSettings()}
}

// For answers the configured transport acting for one caller's token.
func (accounts ExternalAccounts) For(token string) ExternalAccounts {
	configured := accounts.FromSettings()
	configured.Token = token
	return configured
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

// Value runs one operation and answers its decoded result (nil for none).
func (accounts ExternalAccounts) Value(operation string, parameters map[string]any) (any, error) {
	raw, err := accounts.Call(operation, parameters)
	if err != nil || len(raw) == 0 {
		return nil, err
	}
	var result any
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, &BoundaryError{Code: "EUNAVAILABLE", Message: "The Go boundary sent malformed JSON"}
	}
	return result, nil
}

// Object runs one operation and answers its result as an object (empty when
// it is not one).
func (accounts ExternalAccounts) Object(operation string, parameters map[string]any) (map[string]any, error) {
	result, err := accounts.Value(operation, parameters)
	object, _ := RecordValues{}.Object(result)
	if object == nil {
		object = map[string]any{}
	}
	return object, err
}

// OnAccount runs one operation for the caller's token on one account (none
// when id is ""), with params, and answers its result object.
func (accounts ExternalAccounts) OnAccount(operation string, id string, params map[string]any) (map[string]any, error) {
	parameters := TextValue{}.Copy(params)
	parameters["token"] = accounts.Token
	if id != "" {
		parameters["id"] = id
	}
	return accounts.Object(operation, parameters)
}

// List answers the caller's connected accounts.
func (accounts ExternalAccounts) List() ([]map[string]any, error) {
	raw, err := accounts.Call("accounts.list", map[string]any{"token": accounts.Token})
	if err != nil {
		return nil, err
	}
	listed := []map[string]any{}
	if json.Unmarshal(raw, &listed) != nil {
		wrapped := map[string][]map[string]any{}
		if err := json.Unmarshal(raw, &wrapped); err != nil {
			return nil, err
		}
		listed = wrapped["accounts"]
	}
	return listed, nil
}

// Ids answers the ids of the accounts the token's caller has connected, or
// the answer that reports the boundary's failure.
func (accounts ExternalAccounts) Ids(token string) ([]string, *http.Response) {
	raw, err := accounts.Call("accounts.list", map[string]any{"token": token})
	if err != nil {
		return nil, accounts.Failure(err)
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

// Status answers the HTTP status of a boundary failure code (0 when err is
// no boundary failure).
func (ExternalAccounts) Status(err error) int {
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

// IsBoundary reports whether err came from the boundary (the former
// AuthBridgeError).
func (ExternalAccounts) IsBoundary(err error) bool {
	var failure *BoundaryError
	return errors.As(err, &failure)
}

// Refusal answers a failed operation as a delivery route's answer value:
// the owner's stable failures keep their marker, anything else is the
// service being unavailable.
func (accounts ExternalAccounts) Refusal(err error) map[string]any {
	var failure *BoundaryError
	if !errors.As(err, &failure) {
		return map[string]any{"error": "External account service unavailable", "unavailable": true}
	}
	markers := map[int]string{401: "unauthorized", 403: "forbidden", 404: "notFound", 409: "conflict"}
	switch status := accounts.Status(err); status {
	case 400:
		return map[string]any{"error": failure.Message}
	case 422:
		return map[string]any{"error": failure.Message, "blocked": []string{"credentials"}}
	case 503:
		return map[string]any{"error": failure.Message, "unavailable": true}
	default:
		return map[string]any{"error": failure.Message, markers[status]: true}
	}
}

// Get answers one connected account of the caller.
func (accounts ExternalAccounts) Get(token string, id string) (map[string]any, error) {
	return accounts.Object("accounts.get", map[string]any{"token": token, "id": id})
}

// Folders answers the folders of one connected account.
func (accounts ExternalAccounts) Folders(token string, id string) (map[string]any, error) {
	return accounts.Object("accounts.folders", map[string]any{"token": token, "id": id})
}

// UnifiedMessages answers the caller's provider message index rows.
func (accounts ExternalAccounts) UnifiedMessages(token string, input map[string]any) (map[string]any, error) {
	parameters := TextValue{}.Copy(input)
	parameters["token"] = token
	return accounts.Object("accounts.unified_messages", parameters)
}

// MessageSource answers one provider message with its RFC 822 source.
func (accounts ExternalAccounts) MessageSource(token string, id string, stableId string) (map[string]any, error) {
	return accounts.Object("accounts.message_source", map[string]any{"token": token, "id": id, "stableId": stableId})
}

// Message answers one provider message's metadata without its source.
func (accounts ExternalAccounts) Message(token string, id string, stableId string) (map[string]any, error) {
	return accounts.Object("accounts.message", map[string]any{"token": token, "id": id, "stableId": stableId})
}

// Mutate queues one provider mutation.
func (accounts ExternalAccounts) Mutate(token string, id string, input map[string]any) (map[string]any, error) {
	parameters := TextValue{}.Copy(input)
	parameters["token"] = token
	parameters["id"] = id
	return accounts.Object("accounts.mutate", parameters)
}

// Mutation answers the state of one queued provider mutation.
func (accounts ExternalAccounts) Mutation(token string, id string, mutationId string) (map[string]any, error) {
	return accounts.Object("accounts.mutation", map[string]any{"token": token, "id": id, "mutationId": mutationId})
}
