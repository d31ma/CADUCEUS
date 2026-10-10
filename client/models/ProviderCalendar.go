type ProviderCalendar struct {
	CalendarID    string `json:"calendarId"`
	CalendarName  string `json:"calendarName"`
	CalendarColor string `json:"calendarColor"`
	AccountLabel  string `json:"accountLabel"`
	AccountColor  string `json:"accountColor"`
	Writable      *bool  `json:"writable"`
}
