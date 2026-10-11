type AccountProgress struct {
	Phase     string   `json:"phase"`
	Completed float64  `json:"completed"`
	Total     *float64 `json:"total"`
}
