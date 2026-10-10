type MailPreferences struct {
	RichText              bool                `json:"richText"`
	Spellcheck            bool                `json:"spellcheck"`
	SpellcheckLanguage    string              `json:"spellcheckLanguage"`
	DefaultSendAction     string              `json:"defaultSendAction"`
	UndoSendDelay         float64             `json:"undoSendDelay"`
	DefaultFrom           string              `json:"defaultFrom"`
	AutoCcBySender        map[string][]string `json:"autoCcBySender"`
	AutoBlindCopyBySender map[string][]string `json:"autoBccBySender"`
}
