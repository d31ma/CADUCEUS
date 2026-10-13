import (
	"bytes"
	"encoding/json"
	"regexp"
)

// McpToolResults shapes tool results as the MCP SDK answered them.
type McpToolResults struct{}

// Succeeded answers a result: lists become {items}, scalars {value}, and
// the structured content is repeated as indented JSON text.
func (results McpToolResults) Succeeded(result any) McpJsonObject {
	structured := results.Structured(result)
	var text bytes.Buffer
	encoder := json.NewEncoder(&text)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(structured)
	content := []any{McpJsonObject{}.With("type", "text").With("text", string(bytes.TrimRight(text.Bytes(), "\n")))}
	return McpJsonObject{}.With("content", content).With("structuredContent", structured)
}

// Failed answers a tool error with its message on one line, at most 500
// characters.
func (McpToolResults) Failed(message string) McpJsonObject {
	cleaned := TextValue{}.Prefix(regexp.MustCompile(`[\r\n]+`).ReplaceAllString(message, " "), 500)
	content := []any{McpJsonObject{}.With("type", "text").With("text", cleaned)}
	return McpJsonObject{}.With("content", content).With("isError", true)
}

// Structured answers result as an object.
func (McpToolResults) Structured(result any) any {
	switch typed := result.(type) {
	case []any:
		return map[string]any{"items": typed}
	case map[string]any:
		return typed
	case nil:
		return map[string]any{"value": nil}
	}
	return map[string]any{"value": result}
}

// Count answers how many results a tool call produced, for the audit trail.
func (McpToolResults) Count(result any) int {
	switch typed := result.(type) {
	case []any:
		return len(typed)
	case map[string]any:
		if items, isList := typed["items"].([]any); isList {
			return len(items)
		}
		if threads, isNumber := (RecordValues{}).Number(typed["updatedThreads"]); isNumber {
			return int(threads)
		}
		return 1
	case nil:
		return 0
	}
	return 1
}
