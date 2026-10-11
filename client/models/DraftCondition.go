type DraftCondition struct {
	Key   float64 `json:"key"`
	Field string  `json:"field"`
	Op    string  `json:"op"`
	Value string  `json:"value"`
	Label string  `json:"label"`
}
