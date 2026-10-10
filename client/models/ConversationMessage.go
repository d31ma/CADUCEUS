type ConversationMessage struct {
	Id                        string            `json:"id"`
	Subject                   string            `json:"subject"`
	Sender                    string            `json:"sender"`
	HeaderFrom                string            `json:"headerFrom"`
	Recipient                 string            `json:"recipient"`
	Folder                    string            `json:"folder"`
	SystemFolder              string            `json:"systemFolder"`
	ReceivedAt                string            `json:"receivedAt"`
	Body                      string            `json:"body"`
	HtmlBody                  string            `json:"htmlBody"`
	InternetMessageId         string            `json:"rfcMessageId"`
	References                []string          `json:"references"`
	ReplyTo                   []string          `json:"replyTo"`
	Attachments               []EmailAttachment `json:"attachments"`
	Expanded                  bool              `json:"expanded"`
	QuoteExpanded             bool              `json:"quoteExpanded"`
	VisibleConversationSender string            `json:"visibleConversationSender"`
	VisibleConversationBody   string            `json:"visibleConversationBody"`
	QuotedConversationBody    string            `json:"quotedConversationBody"`
	HasConversationQuote      bool              `json:"hasConversationQuote"`
	ConversationBody          string            `json:"conversationBody"`
	FindSenderSegments        []FindSegment     `json:"findSenderSegments"`
	FindRecipientSegments     []FindSegment     `json:"findRecipientSegments"`
	FindSubjectSegments       []FindSegment     `json:"findSubjectSegments"`
	FindBodySegments          []FindSegment     `json:"findBodySegments"`
}
