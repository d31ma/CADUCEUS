// One message row of the list, with everything the view shows.
type MailRow struct {
	ID             string   `json:"id"`
	Sender         string   `json:"sender"`
	Subject        string   `json:"subject"`
	Folder         string   `json:"folder"`
	Read           bool     `json:"read"`
	Starred        bool     `json:"starred"`
	Labels         []string `json:"labels"`
	HasAttachments bool     `json:"hasAttachments"`
	Spam           bool     `json:"spam"`
	SpamScoreLabel string   `json:"spamScoreLabel"`
	SnoozeLabel    string   `json:"snoozeLabel"`
	TimeLabel      string   `json:"timeLabel"`
	Initials       string   `json:"initials"`
	BulkSelected   bool     `json:"bulkSelected"`
	TabIndex       int      `json:"tabIndex"`
}
