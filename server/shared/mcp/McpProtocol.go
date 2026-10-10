import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// McpProtocol answers the stateless JSON-RPC exchange of the MCP endpoint
// as the JavaScript MCP SDK did in its legacy stateless mode with JSON
// responses: each answer is one "message" event of a text/event-stream
// body, and a body of notifications only is accepted with 202.
type McpProtocol struct {
	// Call answers the tools/call result of one tool with parsed arguments.
	Call func(tool McpTool, input map[string]any) McpJsonObject
}

// Answer handles the parsed request body.
func (protocol McpProtocol) Answer(body any, header http.Header) *http.Response {
	accept := header.Get("Accept")
	if !strings.Contains(accept, "application/json") || !strings.Contains(accept, "text/event-stream") {
		return protocol.failure(http.StatusNotAcceptable, "application/json", -32000, "Not Acceptable: Client must accept both application/json and text/event-stream", nil)
	}
	if version := header.Get("Mcp-Protocol-Version"); version != "" && !(PreferenceChoices{}).Includes(protocol.versions(), version) {
		return protocol.failure(http.StatusBadRequest, "application/json", -32000, fmt.Sprintf("Bad Request: Unsupported protocol version: %s (supported versions: %s)", version, strings.Join(protocol.versions(), ", ")), nil)
	}
	messages, batch := body.([]any)
	if !batch {
		messages = []any{body}
	}
	if batch && len(messages) == 0 {
		return protocol.failure(http.StatusBadRequest, protocol.jsonType(), -32600, "Bad Request: empty JSON-RPC batch", nil)
	}
	for _, message := range messages {
		if valid, id := protocol.valid(message); !valid {
			if batch {
				return protocol.failure(http.StatusBadRequest, protocol.jsonType(), -32600, "Bad Request: JSON-RPC batch contains an invalid message", nil)
			}
			return protocol.failure(http.StatusBadRequest, protocol.jsonType(), -32600, "Bad Request: the request body is not a valid JSON-RPC message", id)
		}
	}
	var events bytes.Buffer
	for _, message := range messages {
		request, _ := message.(map[string]any)
		if _, isRequest := request["method"].(string); !isRequest || request["id"] == nil {
			continue
		}
		encoded, _ := json.Marshal(protocol.respond(request))
		events.WriteString("event: message\ndata: " + string(encoded) + "\n\n")
	}
	if events.Len() == 0 {
		return &http.Response{StatusCode: http.StatusAccepted, Header: http.Header{}, Body: http.NoBody}
	}
	events.WriteString("\n")
	header = http.Header{"Content-Type": {"text/event-stream"}, "X-Accel-Buffering": {"no"}}
	return &http.Response{StatusCode: http.StatusOK, Header: header, Body: io.NopCloser(bytes.NewReader(events.Bytes())), ContentLength: int64(events.Len())}
}

// valid reports whether message is a JSON-RPC 2.0 request, notification or
// response, and answers its id for the error of an invalid one.
func (McpProtocol) valid(message any) (bool, any) {
	object, isObject := message.(map[string]any)
	if !isObject {
		return false, nil
	}
	id, hasId := object["id"]
	switch id.(type) {
	case string, float64:
	default:
		id = nil
	}
	if object["jsonrpc"] != "2.0" {
		return false, id
	}
	_, hasResult := object["result"]
	_, hasError := object["error"]
	if _, isMethod := object["method"].(string); isMethod {
		return !hasId || id != nil, id
	}
	return (hasResult || hasError) && id != nil, id
}

func (protocol McpProtocol) respond(request map[string]any) McpJsonObject {
	id := request["id"]
	params, _ := request["params"].(map[string]any)
	switch request["method"] {
	case "initialize":
		return protocol.result(id, protocol.initialized(params))
	case "ping":
		return protocol.result(id, McpJsonObject{})
	case "tools/list":
		return protocol.result(id, McpToolCatalog{}.Listing())
	case "tools/call":
		return protocol.call(id, request["params"])
	}
	return protocol.error(id, -32601, "Method not found")
}

func (protocol McpProtocol) initialized(params map[string]any) McpJsonObject {
	version, _ := params["protocolVersion"].(string)
	if !(PreferenceChoices{}).Includes(protocol.versions(), version) {
		version = protocol.versions()[0]
	}
	capabilities := McpJsonObject{}.With("tools", McpJsonObject{}.With("listChanged", true))
	server := McpJsonObject{}.With("name", "caduceus-mail").With("version", "1.0.0")
	return McpJsonObject{}.With("protocolVersion", version).With("capabilities", capabilities).With("serverInfo", server)
}

func (protocol McpProtocol) call(id any, value any) McpJsonObject {
	params, _ := value.(map[string]any)
	issues := []any{}
	name, isName := params["name"].(string)
	if !isName {
		issues = append(issues, protocol.issue("string", "name", params["name"], protocol.has(params, "name")))
	}
	input := map[string]any{}
	if raw, present := params["arguments"]; present {
		arguments, isObject := raw.(map[string]any)
		if !isObject {
			issues = append(issues, protocol.issue("record", "arguments", raw, true))
		}
		input = arguments
	}
	if len(issues) > 0 {
		text, _ := json.MarshalIndent(issues, "", "  ")
		return protocol.error(id, -32602, "Invalid tools/call request: "+string(text))
	}
	tool, found := McpToolCatalog{}.Find(name)
	if !found {
		return protocol.error(id, -32602, fmt.Sprintf("Tool %s not found", name))
	}
	parsed, problems := McpArguments{}.Parsed(tool, input)
	if len(problems) > 0 {
		return protocol.result(id, McpToolResults{}.Failed(fmt.Sprintf("Input validation error: Invalid arguments for tool %s: %s", name, strings.Join(problems, ", "))))
	}
	return protocol.result(id, protocol.Call(tool, parsed))
}

func (McpProtocol) has(params map[string]any, key string) bool {
	_, present := params[key]
	return present
}

// issue answers one zod issue of the tools/call parameters.
func (McpProtocol) issue(expected string, field string, value any, present bool) McpJsonObject {
	received := McpArguments{}.received(value, present)
	return McpJsonObject{}.
		With("expected", expected).
		With("code", "invalid_type").
		With("path", []string{"params", field}).
		With("message", fmt.Sprintf("Invalid input: expected %s, received %s", expected, received))
}

func (McpProtocol) result(id any, result McpJsonObject) McpJsonObject {
	return McpJsonObject{}.With("result", result).With("jsonrpc", "2.0").With("id", id)
}

func (McpProtocol) error(id any, code int, message string) McpJsonObject {
	return McpJsonObject{}.With("jsonrpc", "2.0").With("id", id).With("error", McpJsonObject{}.With("code", code).With("message", message))
}

func (protocol McpProtocol) failure(status int, contentType string, code int, message string, id any) *http.Response {
	encoded, _ := json.Marshal(McpJsonObject{}.With("jsonrpc", "2.0").With("error", McpJsonObject{}.With("code", code).With("message", message)).With("id", id))
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": {contentType}}, Body: io.NopCloser(bytes.NewReader(encoded)), ContentLength: int64(len(encoded))}
}

func (McpProtocol) jsonType() string { return "application/json;charset=utf-8" }

func (McpProtocol) versions() []string {
	return []string{"2025-11-25", "2025-06-18", "2025-03-26", "2024-11-05", "2024-10-07"}
}
