type ThreadRow struct {
	MessageId  string            `json:"messageId"`
	Initials   string            `json:"initials"`
	AuthorName string            `json:"authorName"`
	Time       string            `json:"time"`
	DateTime   string            `json:"dateTime"`
	Body       string            `json:"body"`
	Blocks     []Block           `json:"blocks"`
	Truncated  bool              `json:"truncated"`
	Edited     bool              `json:"edited"`
	Deleted    bool              `json:"deleted"`
	Reactions  []MessageReaction `json:"reactions"`
	Label      string            `json:"label"`
	IsRoot     bool              `json:"isRoot"`
}
