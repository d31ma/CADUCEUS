type AgendaDay struct {
	Date         string         `json:"date"`
	Label        string         `json:"label"`
	IsToday      bool           `json:"isToday"`
	Events       []CalendarChip `json:"events"`
	AllDayEvents []CalendarChip `json:"allDayEvents"`
	TimedEvents  []CalendarChip `json:"timedEvents"`
}
