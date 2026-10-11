type UserPreferences struct {
	MailboxNames              map[string]string `json:"mailboxNames"`
	InterfaceFont             string            `json:"interfaceFont"`
	ComposerFont              string            `json:"composerFont"`
	NotificationSounds        map[string]string `json:"notificationSounds"`
	ThemePreferences          *ThemePreferences `json:"themePreferences"`
	InterfaceLocale           string            `json:"interfaceLocale"`
	UnreadBadgeScope          string            `json:"unreadBadgeScope"`
	UnreadBadgeDomain         string            `json:"unreadBadgeDomain"`
	KeyboardShortcutOverrides map[string]string `json:"keyboardShortcutOverrides"`
	MailPreferences           *MailPreferences  `json:"mailPreferences"`
	GrammarCheckEnabled       bool              `json:"grammarCheckEnabled"`
	GrammarCheckLanguage      string            `json:"grammarCheckLanguage"`
	GrammarCheckDisabledRules []string          `json:"grammarCheckDisabledRules"`
}
