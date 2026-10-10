type RelatedMessage struct {
	Id         string `json:"id"`
	Subject    string `json:"subject"`
	Snippet    string `json:"snippet"`
	Body       string `json:"body"`
	Sender     string `json:"sender"`
	ReceivedAt string `json:"receivedAt"`
	ThreadId   string `json:"threadId"`
	MessageId  string `json:"messageId"`
}
