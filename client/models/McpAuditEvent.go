type McpAuditEvent struct {
	ToolName      string   `json:"toolName"`
	Status        string   `json:"status"`
	DurationMs    float64  `json:"durationMs"`
	ResultCount   float64  `json:"resultCount"`
	CreatedAt     string   `json:"createdAt"`
	ParameterKeys []string `json:"parameterKeys"`
}
