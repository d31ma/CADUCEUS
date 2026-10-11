type DraftAction struct {
	Key    float64    `json:"key"`
	Action RuleAction `json:"action"`
	Label  string     `json:"label"`
}
