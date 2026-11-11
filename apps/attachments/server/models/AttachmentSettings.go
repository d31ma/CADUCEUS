type AttachmentSettings struct {
	Adapter   string        `json:"adapter"`
	Folder    string        `json:"folder"`
	Bucket    string        `json:"bucket"`
	Region    string        `json:"region"`
	Endpoint  string        `json:"endpoint"`
	Public    string        `json:"public"`
	Messaging string        `json:"messaging"`
	Link      LinkSettings  `json:"link"`
	Limit     LimitSettings `json:"limit"`
}
