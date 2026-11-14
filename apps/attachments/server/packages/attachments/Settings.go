package attachments

type Settings struct {
	Adapter   string        `json:"adapter"`
	Folder    string        `json:"folder"`
	Bucket    string        `json:"bucket"`
	Messaging string        `json:"messaging"`
	Link      LinkSettings  `json:"link"`
	Limit     LimitSettings `json:"limit"`
}
