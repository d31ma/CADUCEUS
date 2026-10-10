// A message summary as the list routes return it.
type Email struct {
	ID                string   `json:"id"`
	Domain            string   `json:"domain"`
	Folder            string   `json:"folder"`
	SystemFolder      string   `json:"systemFolder"`
	Sender            string   `json:"sender"`
	Subject           string   `json:"subject"`
	Snippet           string   `json:"snippet"`
	Read              bool     `json:"read"`
	Starred           bool     `json:"starred"`
	Labels            []string `json:"labels"`
	ReceivedAt        string   `json:"receivedAt"`
	HasAttachments    bool     `json:"hasAttachments"`
	SpamScore         *float64 `json:"spamScore"`
	SpamReasons       []string `json:"spamReasons"`
	SnoozedUntil      string   `json:"snoozedUntil"`
	ProviderAccountID string   `json:"providerAccountId"`
	ThreadID          string   `json:"threadId"`
	To                any      `json:"to"`
	Cc                any      `json:"cc"`
}
