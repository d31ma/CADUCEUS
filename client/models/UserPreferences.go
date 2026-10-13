type UserPreferences struct {
	MailboxNames                        map[string]string `json:"mailboxNames"`
	InterfaceFont                       string            `json:"interfaceFont"`
	ComposerFont                        string            `json:"composerFont"`
	NotificationSounds                  map[string]string `json:"notificationSounds"`
	ThemePreferences                    *ThemePreferences `json:"themePreferences"`
	InterfaceLocale                     string            `json:"interfaceLocale"`
	UnreadBadgeScope                    string            `json:"unreadBadgeScope"`
	UnreadBadgeDomain                   string            `json:"unreadBadgeDomain"`
	KeyboardShortcutOverrides           map[string]string `json:"keyboardShortcutOverrides"`
	MailPreferences                     *MailPreferences  `json:"mailPreferences"`
	CalendarVisibility                  string            `json:"calendarVisibility"`
	RemoteImagePolicy                   string            `json:"remoteImagePolicy"`
	TrustedImageSenders                 []string          `json:"trustedImageSenders"`
	GithubProfileEnrichment             bool              `json:"githubProfileEnrichment"`
	NewMailNotificationDomains          []string          `json:"newMailNotificationDomains"`
	NewMailNotificationExternalAccounts []string          `json:"newMailNotificationExternalAccounts"`
	NewMailNotificationPrivacy          string            `json:"newMailNotificationPrivacy"`
	NewMailNotificationActions          *bool             `json:"newMailNotificationActions"`
	GrammarCheckEnabled                 bool              `json:"grammarCheckEnabled"`
	GrammarCheckLanguage                string            `json:"grammarCheckLanguage"`
	GrammarCheckDisabledRules           []string          `json:"grammarCheckDisabledRules"`
}
