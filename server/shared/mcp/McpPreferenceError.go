// McpPreferenceError refuses an MCP configuration change; Forbidden marks a
// change that would reach beyond the user's assigned domains.
type McpPreferenceError struct {
	Message   string
	Forbidden bool
}

func (failure *McpPreferenceError) Error() string { return failure.Message }
