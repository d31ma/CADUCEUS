type MailPreferences struct {
	MarkAsReadDelay           int    `json:"markAsReadDelay"`
	DetailedHeaders           bool   `json:"detailedHeaders"`
	DetailedNames             bool   `json:"detailedNames"`
	ConversationOrder         string `json:"conversationOrder"`
	EmailRenderMode           string `json:"emailRenderMode"`
	DefaultReplyAction        string `json:"defaultReplyAction"`
	DisplayAttachmentPreviews bool   `json:"displayAttachmentPreviews"`
}
