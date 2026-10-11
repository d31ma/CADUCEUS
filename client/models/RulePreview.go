type RulePreview struct {
	Matched        bool         `json:"matched"`
	PlannedActions []RuleAction `json:"plannedActions"`
}
