type EventForm struct {
	Title       string `json:"title"`
	Start       string `json:"start"`
	End         string `json:"end"`
	AllDay      bool   `json:"allDay"`
	Location    string `json:"location"`
	Description string `json:"description"`
	Repeat      string `json:"repeat"`
	Recurrence  string `json:"recurrence"`
	TimeZone    string `json:"timeZone"`
	CalendarID  string `json:"calendarId"`
	Error       string `json:"error"`
	Saving      bool   `json:"saving"`
}
