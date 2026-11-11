type ThreadRow struct {
	MessageId  string `json:"messageId"`
	Initials   string `json:"initials"`
	AuthorName string `json:"authorName"`
	Time       string `json:"time"`
	DateTime   string `json:"dateTime"`
	Body       string `json:"body"`
	Edited     bool   `json:"edited"`
	Deleted    bool   `json:"deleted"`
	Reactions  string `json:"reactions"`
	Label      string `json:"label"`
}
