type McpFolderRow struct {
	Domain         string `json:"domain"`
	Path           string `json:"path"`
	Role           string `json:"role"`
	AccountAllowed bool   `json:"accountAllowed"`
	Excluded       bool   `json:"excluded"`
}
