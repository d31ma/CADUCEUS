type PreferencesLoaded struct {
	MailboxNames              map[string]string `json:"mailboxNames"`
	InterfaceFont             string            `json:"interfaceFont"`
	ComposerFont              string            `json:"composerFont"`
	NotificationSounds        map[string]string `json:"notificationSounds"`
	InterfaceLocale           string            `json:"interfaceLocale"`
	UnreadBadgeScope          string            `json:"unreadBadgeScope"`
	UnreadBadgeDomain         string            `json:"unreadBadgeDomain"`
	KeyboardShortcutOverrides map[string]string `json:"keyboardShortcutOverrides"`
	ShowUnreadForAllFolders   bool              `json:"showUnreadForAllFolders"`
	Use24HourClock            bool              `json:"use24HourClock"`
}
