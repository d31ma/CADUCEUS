import (
	"encoding/json"
	"errors"
	"strings"
)

// MailActionEntries finds mail action journal entries by document id, by
// the logical id of historical FYLO roots, and by client request.
type MailActionEntries struct{}

// Owned reports whether an entry is an object that belongs to the owner
// ("" matches every owner).
func (MailActionEntries) Owned(entry any, ownerEmail string) bool {
	fields, isObject := entry.(map[string]any)
	if !isObject {
		return false
	}
	if ownerEmail == "" {
		return true
	}
	stored, isText := fields["ownerEmail"].(string)
	return isText && strings.ToLower(stored) == strings.ToLower(ownerEmail)
}

// Fetched answers the entry a getDoc answer holds for id: the document
// under id, else under "identifier", else the first one (nil for none). An
// answer that is not documents holds no entry; a failed operation fails.
func (MailActionEntries) Fetched(documents []FyloDocument, err error, id string) (any, error) {
	var failure *BoundaryError
	if errors.As(err, &failure) {
		return nil, err
	}
	decoded := map[string]any{}
	order := []string{}
	for _, document := range documents {
		var value any
		_ = json.Unmarshal(document.Data, &value)
		decoded[document.Id] = value
		order = append(order, document.Id)
	}
	for _, key := range []string{id, "identifier"} {
		if value := decoded[key]; value != nil {
			return value, nil
		}
	}
	if len(order) == 0 {
		return nil, nil
	}
	return decoded[order[0]], nil
}

// Matching answers the first record whose document id or logical id is id
// and that belongs to the owner, or an empty record.
func (entries MailActionEntries) Matching(records []FyloRecord, id string, ownerEmail string) FyloRecord {
	for _, record := range records {
		if (record.Id == id || record.Data["id"] == id) && entries.Owned(record.Data, ownerEmail) {
			return record
		}
	}
	return FyloRecord{}
}

// OwnedBy answers the records whose owner is ownerEmail, ignoring case.
func (entries MailActionEntries) OwnedBy(records []FyloRecord, ownerEmail string) []FyloRecord {
	owned := []FyloRecord{}
	for _, record := range records {
		stored, isText := record.Data["ownerEmail"].(string)
		if isText && strings.ToLower(stored) == strings.ToLower(ownerEmail) {
			owned = append(owned, record)
		}
	}
	return owned
}

// ForClientRequest answers the first record made by one client request, or
// an empty record.
func (MailActionEntries) ForClientRequest(records []FyloRecord, clientRequestId string) FyloRecord {
	for _, record := range records {
		if record.Data["clientRequestId"] == clientRequestId {
			return record
		}
	}
	return FyloRecord{}
}
