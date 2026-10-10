type OpenMessageChanged struct {
	EmailId            string          `json:"emailId"`
	Domain             string          `json:"domain"`
	Folder             string          `json:"folder"`
	SystemFolder       string          `json:"systemFolder"`
	Labels             []string        `json:"labels"`
	Subject            string          `json:"subject"`
	ParticipantAddress string          `json:"participantAddress"`
	Standalone         bool            `json:"standalone"`
	CalendarInvite     *CalendarInvite `json:"calendarInvite"`
}
