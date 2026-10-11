type PreferencesChosen struct {
	InterfaceFont    string            `json:"interfaceFont"`
	ComposerFont     string            `json:"composerFont"`
	ThemePreferences *ThemePreferences `json:"themePreferences"`
	InterfaceLocale  string            `json:"interfaceLocale"`
	MailPreferences  *MailPreferences  `json:"mailPreferences"`
	AccentColor      string            `json:"accentColor"`
}
