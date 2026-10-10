type OfflineWriteRequested struct {
	Kind        string         `json:"kind"`
	Path        string         `json:"path"`
	Method      string         `json:"method"`
	Body        any            `json:"body"`
	Message     string         `json:"message"`
	CoalesceKey string         `json:"coalesceKey"`
	Metadata    map[string]any `json:"metadata"`
}
