type CalendarInvite struct {
	Id                 string           `json:"id"`
	Title              string           `json:"title"`
	Start              string           `json:"start"`
	End                string           `json:"end"`
	AllDay             bool             `json:"allDay"`
	Location           string           `json:"location"`
	Organizer          string           `json:"organizer"`
	Status             string           `json:"status"`
	MyStatus           string           `json:"myStatus"`
	CanRespond         bool             `json:"canRespond"`
	IsRecurring        bool             `json:"isRecurring"`
	RecurrenceId       string           `json:"recurrenceId"`
	SenderResponse     *InviteResponse  `json:"senderResponse"`
	Attachment         *EmailAttachment `json:"attachment"`
	DisplayStatus      string           `json:"displayStatus"`
	StatusLabel        string           `json:"statusLabel"`
	SenderResponseText string           `json:"senderResponseText"`
}
