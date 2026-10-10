// McpTool is one tool the MCP endpoint lists: what it does, the access
// category it needs (read, write or send) and its arguments in order.
type McpTool struct {
	Name        string
	Category    string
	Description string
	Fields      []McpToolField
}
