type TranslationRestoreRequested struct {
	MessageId string `json:"messageId"`
	Target    string `json:"target"`
	Detect    bool   `json:"detect"`
}
