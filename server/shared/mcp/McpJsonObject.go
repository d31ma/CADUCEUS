import (
	"bytes"
	"encoding/json"
)

// McpJsonObject is a JSON object that keeps its members in the order they
// were added, as the JavaScript MCP server wrote them.
type McpJsonObject []McpJsonField

// With answers the object with one more member.
func (object McpJsonObject) With(key string, value any) McpJsonObject {
	return append(object, McpJsonField{Key: key, Value: value})
}

// MarshalJSON writes the members in order, without HTML escaping.
func (object McpJsonObject) MarshalJSON() ([]byte, error) {
	var buffer bytes.Buffer
	buffer.WriteByte('{')
	for index, field := range object {
		if index > 0 {
			buffer.WriteByte(',')
		}
		key, err := json.Marshal(field.Key)
		if err != nil {
			return nil, err
		}
		buffer.Write(key)
		buffer.WriteByte(':')
		var value bytes.Buffer
		encoder := json.NewEncoder(&value)
		encoder.SetEscapeHTML(false)
		if err := encoder.Encode(field.Value); err != nil {
			return nil, err
		}
		buffer.Write(bytes.TrimRight(value.Bytes(), "\n"))
	}
	buffer.WriteByte('}')
	return buffer.Bytes(), nil
}
