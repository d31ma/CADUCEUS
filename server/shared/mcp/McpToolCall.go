// McpToolCall is one tool call of an authenticated capability: the tool, its
// parsed arguments and the request's database.
type McpToolCall struct {
	Principal McpPrincipal
	Name      string
	Input     map[string]any
	Fylo      *FyloHelper
}
