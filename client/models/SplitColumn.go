type SplitColumn struct {
	Owner  string         `json:"owner"`
	Events []CalendarChip `json:"events"`
}
