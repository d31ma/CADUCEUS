type CalendarCell struct {
	Date      string         `json:"date"`
	DayLabel  string         `json:"dayLabel"`
	ClassName string         `json:"className"`
	Events    []CalendarChip `json:"events"`
}
