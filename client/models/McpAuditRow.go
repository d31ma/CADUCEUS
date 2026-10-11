type McpAuditRow struct {
	ToolName       string  `json:"toolName"`
	Status         string  `json:"status"`
	DurationMs     float64 `json:"durationMs"`
	ResultCount    float64 `json:"resultCount"`
	CreatedAtLabel string  `json:"createdAtLabel"`
	ParameterLabel string  `json:"parameterLabel"`
}
