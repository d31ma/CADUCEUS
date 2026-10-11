// OpenMessageChanged (retained) is the message the reader shows; the
// conversation, contact, snooze, share and invitation parts follow it.
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
	Account            string          `json:"account"`
}
