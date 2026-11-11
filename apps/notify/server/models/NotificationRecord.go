type NotificationRecord struct {
	Id                string `json:"id,omitempty"`
	DomainId          string `json:"domainId"`
	MemberId          string `json:"memberId"`
	Kind              string `json:"kind"`
	ConversationId    string `json:"conversationId"`
	MessageId         string `json:"messageId"`
	ThreadRootId      string `json:"threadRootId"`
	AuthorDisplayName string `json:"authorDisplayName"`
	DedupeKey         string `json:"dedupeKey"`
	CreatedAt         string `json:"createdAt"`
	ReadAt            string `json:"readAt"`
	PushState         string `json:"pushState"`
	PushAttempts      int    `json:"pushAttempts"`
	SummaryDueAt      int64  `json:"summaryDueAt"`
	SummaryState      string `json:"summaryState"`
	SummarizedAt      string `json:"summarizedAt"`
}
