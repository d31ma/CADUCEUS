type MailPreferences struct {
	WorkspaceLayout           string              `json:"workspaceLayout"`
	Use24HourClock            bool                `json:"use24HourClock"`
	InterfaceScale            float64             `json:"interfaceScale"`
	ShowUnreadForAllFolders   bool                `json:"showUnreadForAllFolders"`
	MarkAsReadDelay           int                 `json:"markAsReadDelay"`
	DetailedHeaders           bool                `json:"detailedHeaders"`
	DetailedNames             bool                `json:"detailedNames"`
	RestrictMessageWidth      bool                `json:"restrictMessageWidth"`
	BackspaceAction           string              `json:"backspaceAction"`
	SwipeEnabled              bool                `json:"swipeEnabled"`
	ConversationOrder         string              `json:"conversationOrder"`
	EmailRenderMode           string              `json:"emailRenderMode"`
	RichText                  bool                `json:"richText"`
	Spellcheck                bool                `json:"spellcheck"`
	SpellcheckLanguage        string              `json:"spellcheckLanguage"`
	DefaultSendAction         string              `json:"defaultSendAction"`
	DefaultReplyAction        string              `json:"defaultReplyAction"`
	UndoSendDelay             int                 `json:"undoSendDelay"`
	DefaultFrom               string              `json:"defaultFrom"`
	AutoCcBySender            map[string][]string `json:"autoCcBySender"`
	AutoBccBySender           map[string][]string `json:"autoBccBySender"`
	DisplayAttachmentPreviews bool                `json:"displayAttachmentPreviews"`
	UnsnoozeToTop             bool                `json:"unsnoozeToTop"`
	FindInMailDisabledDomains []string            `json:"findInMailDisabledDomains"`
}
