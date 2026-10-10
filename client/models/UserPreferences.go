type UserPreferences struct {
	GrammarCheckEnabled       bool            `json:"grammarCheckEnabled"`
	GrammarCheckLanguage      string          `json:"grammarCheckLanguage"`
	GrammarCheckDisabledRules []string        `json:"grammarCheckDisabledRules"`
	MailPreferences           MailPreferences `json:"mailPreferences"`
}
