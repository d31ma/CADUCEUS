import (
	"encoding/json"
	"fmt"
)

// McpAuditEventsRecords reads MCP audit events. Each event stores the names
// of the parameters a tool call used as JSON text.
type McpAuditEventsRecords struct{}

// RetainedEvents is how many events a mailbox keeps.
func (McpAuditEventsRecords) RetainedEvents() int { return 100 }

// ReturnedEvents is how many events a listing answers by default.
func (McpAuditEventsRecords) ReturnedEvents() int { return 50 }

// ParameterKeys answers the parameter names stored as JSON text, keeping
// only text items; anything unreadable answers none.
func (McpAuditEventsRecords) ParameterKeys(value any) []string {
	text := "[]"
	if (RecordValues{}).Truthy(value) {
		text = fmt.Sprint(value)
	}
	var parsed []any
	keys := []string{}
	if json.Unmarshal([]byte(text), &parsed) != nil {
		return keys
	}
	for _, item := range parsed {
		if key, isText := item.(string); isText {
			keys = append(keys, key)
		}
	}
	return keys
}

// CreatedAtText answers String(event.createdAt), the text events sort by.
func (McpAuditEventsRecords) CreatedAtText(event map[string]any) string {
	value, present := event["createdAt"]
	if !present {
		return "undefined"
	}
	if value == nil {
		return "null"
	}
	return fmt.Sprint(value)
}
