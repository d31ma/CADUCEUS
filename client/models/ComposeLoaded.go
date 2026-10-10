type ComposeLoaded struct {
	Reason          string              `json:"reason"`
	From            string              `json:"from"`
	To              string              `json:"to"`
	Cc              string              `json:"cc"`
	BlindCopy       string              `json:"bcc"`
	ReplyTo         string              `json:"replyTo"`
	Subject         string              `json:"subject"`
	InReplyTo       string              `json:"inReplyTo"`
	References      []string            `json:"references"`
	ScheduledID     string              `json:"scheduledId"`
	SendAt          string              `json:"sendAt"`
	SourceEmailID   string              `json:"sourceEmailId"`
	RemindAt        string              `json:"remindAt"`
	OpenTracking    bool                `json:"openTracking"`
	LinkTracking    bool                `json:"linkTracking"`
	DraftID         string              `json:"draftId"`
	DraftClientID   string              `json:"draftClientId"`
	DraftVersion    int                 `json:"draftVersion"`
	DraftStatus     string              `json:"draftStatus"`
	SavedSnapshot   bool                `json:"savedSnapshot"`
	ExplicitPrefill bool                `json:"explicitPrefill"`
	Recovered       bool                `json:"recovered"`
	Meaningful      bool                `json:"meaningful"`
	SignatureID     string              `json:"signatureId"`
	Attachments     []ComposeAttachment `json:"attachments"`
}
