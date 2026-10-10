import "encoding/json"

// DomainRecords converts domain documents. FYLO stores a domain's routes as
// JSON text; routes see them as a list. Other fields pass through unchanged.
type DomainRecords struct{}

// Decoded answers one stored domain document with its routes as a list.
func (DomainRecords) Decoded(raw json.RawMessage) map[string]any {
	document := StoredRecords{}.Decoded(raw)
	if text, isText := document["routes"].(string); isText {
		var routes any
		_ = json.Unmarshal([]byte(text), &routes)
		document["routes"] = routes
	} else if document["routes"] == nil {
		document["routes"] = []any{}
	}
	return document
}

// Encoded answers a domain document with its routes as JSON text, as
// JSON.stringify wrote them (a missing list stays missing).
func (DomainRecords) Encoded(config map[string]any) map[string]any {
	stored := StoredRecords{}.Without(config, "routes")
	if routes, present := config["routes"]; present {
		stored["routes"] = StoredRecords{}.JsonText(routes)
	}
	return stored
}
