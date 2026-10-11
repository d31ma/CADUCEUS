type CalendarEvent struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Start        string `json:"start"`
	End          string `json:"end"`
	AllDay       bool   `json:"allDay"`
	Location     string `json:"location"`
	Description  string `json:"description"`
	Organizer    string `json:"organizer"`
	Source       string `json:"source"`
	Recurrence   string `json:"recurrence"`
	TimeZone     string `json:"timeZone"`
	SeriesID     string `json:"seriesId"`
	RecurrenceID string `json:"recurrenceId"`
	IsRecurring  bool   `json:"isRecurring"`
	IsException  bool   `json:"isException"`
	ReadOnly     bool   `json:"readOnly"`
	CalendarID   string `json:"calendarId"`
	AccountLabel string `json:"accountLabel"`
	CalendarName string `json:"calendarName"`
	OwnerEmail   string `json:"ownerEmail"`
	UserEmail    string `json:"userEmail"`
	Owner        string `json:"owner"`
	OwnerLabel   string `json:"ownerLabel"`
}
