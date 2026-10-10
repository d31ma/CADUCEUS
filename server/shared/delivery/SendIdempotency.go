import (
	"encoding/json"
	"sort"
	"strings"
)

// SendIdempotency is the durable exactly-once acceptance journal for
// outbound mail: a send that names an idempotencyKey replays its first
// accepted answer, refuses reuse for a different message, and is never
// retried automatically once its outcome is uncertain. The read-then-create
// step holds a process lock, as the JavaScript server serialized it.
type SendIdempotency struct {
	Requests *SendRequestsHelper
}

// Hash answers a stable hash of a request, independent of field order.
func (idempotency SendIdempotency) Hash(value any) string {
	return Security{}.Sha256Hex(idempotency.canonical(value))
}

// Inspect answers ("new", nil) or ("replay", result) without creating a
// journal row, so a request that fails validation stays retryable; a
// conflict is a *DeliveryRefusal.
func (idempotency SendIdempotency) Inspect(ownerEmail string, key string, requestHash string) (string, any, error) {
	documentId, record, err := idempotency.Requests.Find(ownerEmail, key)
	if err != nil || documentId == "" || record == nil {
		return "new", nil, err
	}
	return idempotency.existing(record, requestHash)
}

// Begin starts the journal row of a new request and answers ("started",
// documentId), or ("replay", result) when another request got there first.
func (idempotency SendIdempotency) Begin(ownerEmail string, key string, requestHash string) (string, any, error) {
	kind, value := "", any(nil)
	err := ProcessLock{Name: "send-requests"}.Held(func() error {
		var err error
		kind, value, err = idempotency.Inspect(ownerEmail, key, requestHash)
		if err != nil || kind != "new" {
			return err
		}
		now := EpochTime{}.Now()
		owner := strings.ToLower(ownerEmail)
		documentId, err := idempotency.Requests.Put(map[string]any{
			"id": "send-request-" + Security{}.RandomUuid(), "ownerEmail": owner, "domain": DeliveryFields{}.Domain(owner),
			"idempotencyKey": key, "requestHash": requestHash, "state": "applying", "resultJson": "", "createdAt": now, "updatedAt": now,
		})
		kind, value = "started", documentId
		return err
	})
	return kind, value, err
}

// Accept commits the exact answer before the route returns it.
func (idempotency SendIdempotency) Accept(documentId string, result any) (any, error) {
	if documentId == "" {
		return result, nil
	}
	err := idempotency.Requests.Update(documentId, map[string]any{"state": "accepted", "resultJson": StoredRecords{}.JsonText(result), "updatedAt": EpochTime{}.Now()})
	return result, err
}

// MarkAmbiguous keeps an uncertain outcome rather than allow a duplicate
// delivery retry.
func (idempotency SendIdempotency) MarkAmbiguous(documentId string) {
	if documentId != "" {
		_ = idempotency.Requests.Update(documentId, map[string]any{"state": "ambiguous", "updatedAt": EpochTime{}.Now()})
	}
}

func (SendIdempotency) existing(record map[string]any, requestHash string) (string, any, error) {
	if record["requestHash"] != requestHash {
		return "", nil, &DeliveryRefusal{Message: "idempotencyKey was already used for a different message", Status: 409, Kind: "mismatch"}
	}
	switch record["state"] {
	case "accepted":
		var result any
		if json.Unmarshal([]byte(RecordValues{}.Text(record["resultJson"])), &result) != nil {
			return "", nil, &DeliveryRefusal{Message: "The accepted send result cannot be recovered safely", Status: 409, Kind: "ambiguous"}
		}
		return "replay", result, nil
	case "ambiguous":
		return "", nil, &DeliveryRefusal{Message: "The previous send outcome is ambiguous and will not be retried automatically", Status: 409, Kind: "ambiguous"}
	}
	return "", nil, &DeliveryRefusal{Message: "The previous send is still being applied", Status: 409, Kind: "applying"}
}

// canonical answers JSON with object keys sorted at every level.
func (idempotency SendIdempotency) canonical(value any) string {
	switch typed := value.(type) {
	case map[string]any:
		keys := []string{}
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		parts := []string{}
		for _, key := range keys {
			parts = append(parts, StoredRecords{}.JsonText(key)+":"+idempotency.canonical(typed[key]))
		}
		return "{" + strings.Join(parts, ",") + "}"
	case []any:
		parts := []string{}
		for _, item := range typed {
			parts = append(parts, idempotency.canonical(item))
		}
		return "[" + strings.Join(parts, ",") + "]"
	}
	return StoredRecords{}.JsonText(value)
}
