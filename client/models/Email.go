// A message summary as the list routes return it.
type Email struct {
	ID                string    `json:"id,omitempty"`
	Domain            string    `json:"domain,omitempty"`
	Folder            string    `json:"folder,omitempty"`
	SystemFolder      string    `json:"systemFolder,omitempty"`
	Sender            string    `json:"sender,omitempty"`
	Subject           string    `json:"subject,omitempty"`
	Snippet           string    `json:"snippet,omitempty"`
	Read              bool      `json:"read,omitempty"`
	Starred           bool      `json:"starred,omitempty"`
	Labels            []string  `json:"labels,omitempty"`
	ReceivedAt        string    `json:"receivedAt,omitempty"`
	HasAttachments    bool      `json:"hasAttachments,omitempty"`
	SpamScore         *float64  `json:"spamScore,omitempty"`
	SpamReasons       *[]string `json:"spamReasons,omitempty"`
	SnoozedUntil      string    `json:"snoozedUntil,omitempty"`
	ProviderAccountID string    `json:"providerAccountId,omitempty"`
	ThreadID          string    `json:"threadId,omitempty"`
	To                any       `json:"to,omitempty"`
	Cc                any       `json:"cc,omitempty"`
}
