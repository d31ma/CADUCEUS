type NotifySettings struct {
	EmailSummaryAfterMinutes int      `json:"emailSummaryAfterMinutes"`
	MutedConversationIds     []string `json:"mutedConversationIds"`
}
