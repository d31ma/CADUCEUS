// One conversation row of the threaded list.
type MailThreadRow struct {
	ID                 string    `json:"id"`
	Subject            string    `json:"subject"`
	Count              int       `json:"count"`
	Sender             string    `json:"sender"`
	Initials           string    `json:"initials"`
	SnoozeLabel        string    `json:"snoozeLabel"`
	TimeLabel          string    `json:"timeLabel"`
	PersonalLevel      int       `json:"personalLevel"`
	PersonalLevelLabel string    `json:"personalLevelLabel"`
	Expanded           bool      `json:"expanded"`
	Selected           bool      `json:"selected"`
	TabIndex           int       `json:"tabIndex"`
	Emails             []MailRow `json:"emails"`
}
