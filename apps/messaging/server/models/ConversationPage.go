type ConversationPage struct {
	Items      []ConversationSummary `json:"items"`
	NextCursor string                `json:"nextCursor"`
}
