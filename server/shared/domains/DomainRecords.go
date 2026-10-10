import "encoding/json"

// DomainRecords converts domain documents. FYLO stores a domain's routes as
// JSON text; routes see them as a list. Other fields pass through unchanged.
type DomainRecords struct{}

// Decoded answers one stored domain document with its routes as a list.
func (DomainRecords) Decoded(raw json.RawMessage) map[string]any {
	document := map[string]any{}
	_ = json.Unmarshal(raw, &document)
	text, isText := document["routes"].(string)
	if isText {
		var routes any
		_ = json.Unmarshal([]byte(text), &routes)
		document["routes"] = routes
	}
	if document["routes"] == nil {
		document["routes"] = []any{}
	}
	return document
}

// Encoded answers a domain document with its routes as JSON text.
func (DomainRecords) Encoded(config map[string]any) map[string]any {
	stored := map[string]any{}
	for key, value := range config {
		stored[key] = value
	}
	routes, _ := json.Marshal(config["routes"])
	stored["routes"] = string(routes)
	return stored
}
