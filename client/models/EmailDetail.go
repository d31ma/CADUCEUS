// EmailDetail is the complete authorized message returned for reader requests.
// Keep Email separate: mailbox and search routes return summaries.
type EmailDetail struct {
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

	// The reader's whole message: GET /inbox/{id}, and the labels it shows.
	HeaderFrom          string            `json:"headerFrom,omitempty"`
	Recipient           string            `json:"recipient,omitempty"`
	OriginalRecipient   string            `json:"originalRecipient,omitempty"`
	ReplyTo             []string          `json:"replyTo,omitempty"`
	InternetMessageID   string            `json:"rfcMessageId,omitempty"`
	MessageID           string            `json:"messageId,omitempty"`
	References          []string          `json:"references,omitempty"`
	Body                string            `json:"body,omitempty"`
	HtmlBody            string            `json:"htmlBody,omitempty"`
	HasRawSource        bool              `json:"hasRawSource,omitempty"`
	OpenedAt            string            `json:"openedAt,omitempty"`
	Attachments         []EmailAttachment `json:"attachments,omitempty"`
	Spam                bool              `json:"spam,omitempty"`
	HasSpamScore        bool              `json:"hasSpamScore,omitempty"`
	SpamScoreLabel      string            `json:"spamScoreLabel,omitempty"`
	ReplyToMismatch     bool              `json:"replyToMismatch,omitempty"`
	OpenTracking        bool              `json:"openTracking,omitempty"`
	LinkTracking        bool              `json:"linkTracking,omitempty"`
	TrackingEvents      []TrackingEvent   `json:"trackingEvents,omitempty"`
	TrackingSummary     string            `json:"trackingSummary,omitempty"`
	GithubUrl           string            `json:"githubUrl,omitempty"`
	UnsubscribeUrl      string            `json:"unsubscribeUrl,omitempty"`
	UnsubscribeOneClick bool              `json:"unsubscribeOneClick,omitempty"`
	UnsubscribeMailto   string            `json:"unsubscribeMailto,omitempty"`
	UnsubscribeBodyUrl  string            `json:"unsubscribeBodyUrl,omitempty"`
	SecurityWarnings    []string          `json:"securityWarnings,omitempty"`
	CalendarInvite      *CalendarInvite   `json:"calendarInvite,omitempty"`
	Opens               []TrackingHit     `json:"opens,omitempty"`
	Clicks              []TrackingHit     `json:"clicks,omitempty"`
	OpenCount           int               `json:"openCount,omitempty"`
	ClickCount          int               `json:"clickCount,omitempty"`
	ReceivedLabel       string            `json:"receivedLabel,omitempty"`
	OpenedLabel         string            `json:"openedLabel,omitempty"`
}
