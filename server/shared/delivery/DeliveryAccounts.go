import (
	"encoding/json"
	"errors"
	"time"
)

// DeliveryAccounts carries the connected-account operations delivery needs
// (accounts.list and the accounts.submit* journal) to the Go owner with the
// caller's bearer token, as the former ExternalAccounts facade did. The
// external-accounts area owns the full client; this is the minimal private
// version delivery needs. Failures answer like accountFailure.
type DeliveryAccounts struct {
	Token string
}

// Call runs one accounts operation with params (the token and account id
// are added) and decodes its result.
func (accounts DeliveryAccounts) Call(operation string, id string, params map[string]any) (map[string]any, error) {
	parameters := DeliveryFields{}.Merged(params, map[string]any{"token": accounts.Token})
	if id != "" {
		parameters["id"] = id
	}
	raw, err := accounts.raw(operation, parameters)
	if err != nil {
		return nil, err
	}
	result := map[string]any{}
	return result, json.Unmarshal(raw, &result)
}

// List answers the caller's connected accounts.
func (accounts DeliveryAccounts) List() ([]map[string]any, error) {
	raw, err := accounts.raw("accounts.list", map[string]any{"token": accounts.Token})
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

func (accounts DeliveryAccounts) raw(operation string, parameters map[string]any) (json.RawMessage, error) {
	encoded, err := json.Marshal(map[string]any{"op": operation, "params": parameters})
	if err != nil {
		return nil, err
	}
	if len(encoded) > accounts.requestLimit() {
		return nil, &BoundaryError{Code: "EBADREQUEST", Message: "Boundary request is too large"}
	}
	return Boundary{}.FromSettings().Call(json.RawMessage(encoded), accounts.responseLimit(), accounts.timeout())
}

// Failure answers a failed accounts operation: the owner's stable failures
// keep their status, anything else is the service being unavailable.
func (DeliveryAccounts) Failure(err error) map[string]any {
	var failure *BoundaryError
	if !errors.As(err, &failure) {
		return map[string]any{"error": "External account service unavailable", "unavailable": true}
	}
	switch failure.Code {
	case "EBADREQUEST", "EUNSUPPORTED":
		return map[string]any{"error": failure.Message}
	case "EUNAUTHORIZED":
		return map[string]any{"error": failure.Message, "unauthorized": true}
	case "EFORBIDDEN", "EPOLICY":
		return map[string]any{"error": failure.Message, "forbidden": true}
	case "ENOTFOUND":
		return map[string]any{"error": failure.Message, "notFound": true}
	case "ECONFLICT":
		return map[string]any{"error": failure.Message, "conflict": true}
	case "EAUTHREQUIRED":
		return map[string]any{"error": failure.Message, "blocked": []string{"credentials"}}
	}
	return map[string]any{"error": failure.Message, "unavailable": true}
}

func (DeliveryAccounts) requestLimit() int { return 47 << 20 }

func (DeliveryAccounts) responseLimit() int { return 64 << 20 }

func (DeliveryAccounts) timeout() time.Duration { return 15 * time.Second }
