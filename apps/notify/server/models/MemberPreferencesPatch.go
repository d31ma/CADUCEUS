type MemberPreferencesPatch struct {
	EmailSummaryAfterMinutes int      `json:"emailSummaryAfterMinutes"`
	MutedConversationIds     []string `json:"mutedConversationIds"`
}
