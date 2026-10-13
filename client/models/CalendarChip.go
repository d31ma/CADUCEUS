type CalendarChip struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	TimeLabel   string `json:"timeLabel"`
	Tooltip     string `json:"tooltip"`
	IsRecurring bool   `json:"isRecurring"`
	ClassName   string `json:"className"`
	Draggable   string `json:"draggable"`
	Start       string `json:"start"`
}
