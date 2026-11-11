package search

type ConversationPage struct {
	Items      []ConversationSummary `json:"items"`
	NextCursor string                `json:"nextCursor"`
}
