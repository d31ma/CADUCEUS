import "encoding/json"

// PreferenceService loads the signed-in user's preferences and applies the
// ones that shape the whole document: theme, accent, fonts, interface locale
// and the mail layout. Go cannot touch <html>, so it publishes the attributes
// in DocumentAttributesChanged and BrowserPageService sets them. Other
// services hear the loaded values in the retained PreferencesLoaded event and
// change them with PreferencesChosen.
// @Service
type PreferenceService struct {
	// @Needs
	usersClient               *UsersClient
	applicationEventPublisher ApplicationEventPublisher
	ThemeToggleLabel          string
	themes                    ThemePreferences
	systemDark                bool
	language                  string
	interfaceFont             string
	composerFont              string
	localeChoice              string
	accent                    string
	mail                      MailPreferences
	token                     string
}

// @OnStart
func (preference *PreferenceService) Start() {
	preference.themes = AppearanceUtils{}.Themes(ThemePreferences{})
	preference.interfaceFont = AppearanceUtils{}.InterfaceFont("")
	preference.composerFont = AppearanceUtils{}.ComposerFont("")
	preference.localeChoice = LocaleUtils{}.Choice("")
	preference.accent = AppearanceUtils{}.Accent("")
	preference.mail = AppearanceUtils{}.DefaultMail()
	preference.apply()
}

// HearStorage restores the choices this tab and device already hold, before
// the server answers.
// @OnEvent({"replay":true})
func (preference *PreferenceService) HearStorage(stored StoredValues) {
	preference.themes = AppearanceUtils{}.StoredThemes(stored.Local[preference.themeKey()], stored.Local["caduceus-theme"])
	preference.interfaceFont = AppearanceUtils{}.InterfaceFont(stored.Session[preference.interfaceFontKey()])
	preference.composerFont = AppearanceUtils{}.ComposerFont(stored.Session[preference.composerFontKey()])
	preference.mail = AppearanceUtils{}.Mail(stored.Session[preference.mailKey()])
	preference.localeChoice = LocaleUtils{}.Choice(stored.Local[preference.localeKey()])
	preference.accent = AppearanceUtils{}.Accent(stored.Local[preference.accentKey()])
	preference.apply()
}

// HearDevice follows the device's colour scheme and language.
// @OnEvent({"replay":true})
func (preference *PreferenceService) HearDevice(changed PreferencesChanged) {
	preference.systemDark = changed.Scheme == AppearanceUtils{}.BaseMode("dark")
	preference.language = changed.Language
	preference.apply()
}

// HearSession loads the signed-in user's preferences.
// @OnEvent({"replay":true})
func (preference *PreferenceService) HearSession(changed SessionChanged) {
	preference.token = changed.Token
	FlowUtils{}.Either(changed.SignedIn, preference.load, preference.signedOut)()
}

// HearChoice applies a preference that the settings page saved.
// @OnEvent
func (preference *PreferenceService) HearChoice(chosen PreferencesChosen) {
	preference.adopt(UserPreferences{InterfaceFont: chosen.InterfaceFont, ComposerFont: chosen.ComposerFont, ThemePreferences: chosen.ThemePreferences, InterfaceLocale: chosen.InterfaceLocale, MailPreferences: chosen.MailPreferences})
	preference.accent = AppearanceUtils{}.Accent(TextUtils{}.FirstOf(chosen.AccentColor, preference.accent))
	preference.persist()
	preference.apply()
}

// ToggleTheme switches between the light and dark sides of the theme
// catalog and saves the choice without waiting on the network.
func (preference *PreferenceService) ToggleTheme() error {
	preference.themes.ActiveTheme = AppearanceUtils{}.Opposite(AppearanceUtils{}.BaseMode(AppearanceUtils{}.Resolved(preference.themes, preference.systemDark)))
	preference.persist()
	preference.apply()
	_, failure := preference.usersClient.SaveTheme(preference.bearer(), ThemeSaved{ThemePreferences: preference.themes})
	return failure
}

func (preference *PreferenceService) load() {
	loaded, failure := preference.usersClient.Preferences(preference.bearer())
	FlowUtils{}.Either(failure == nil, func() { preference.adoptLoaded(loaded) }, func() { preference.announce(UserPreferences{}) })()
}

func (preference *PreferenceService) adoptLoaded(loaded UserPreferences) {
	preference.adopt(loaded)
	preference.persist()
	preference.apply()
	preference.announce(loaded)
	preference.overrides(AppearanceUtils{}.Merged(map[string]string{}, loaded.KeyboardShortcutOverrides))
}

func (preference *PreferenceService) signedOut() {
	preference.overrides(map[string]string{})
}

func (preference *PreferenceService) overrides(overrides map[string]string) {
	preference.applicationEventPublisher.PublishEvent(KeyboardOverridesChanged{Overrides: overrides}, true)
}

func (preference *PreferenceService) bearer() string {
	return "Bearer " + preference.token
}

func (preference *PreferenceService) adopt(loaded UserPreferences) {
	preference.interfaceFont = AppearanceUtils{}.InterfaceFont(TextUtils{}.FirstOf(loaded.InterfaceFont, preference.interfaceFont))
	preference.composerFont = AppearanceUtils{}.ComposerFont(TextUtils{}.FirstOf(loaded.ComposerFont, preference.composerFont))
	preference.localeChoice = LocaleUtils{}.Choice(TextUtils{}.FirstOf(loaded.InterfaceLocale, preference.localeChoice))
	preference.themes = preference.chosenThemes(loaded.ThemePreferences)
	preference.mail = preference.chosenMail(loaded.MailPreferences)
}

func (preference *PreferenceService) announce(loaded UserPreferences) {
	preference.applicationEventPublisher.PublishEvent(PreferencesLoaded{
		MailboxNames: loaded.MailboxNames, InterfaceFont: preference.interfaceFont, ComposerFont: preference.composerFont,
		NotificationSounds: loaded.NotificationSounds, InterfaceLocale: preference.localeChoice,
		UnreadBadgeScope: loaded.UnreadBadgeScope, UnreadBadgeDomain: loaded.UnreadBadgeDomain,
		KeyboardShortcutOverrides: loaded.KeyboardShortcutOverrides,
		ShowUnreadForAllFolders:   preference.mail.ShowUnreadForAllFolders, Use24HourClock: preference.mail.Use24HourClock,
	}, true)
}

func (preference *PreferenceService) persist() {
	themes, _ := json.Marshal(preference.themes)
	mail, _ := json.Marshal(preference.mail)
	preference.applicationEventPublisher.PublishEvent(StorageWritten{Area: "local", Values: map[string]string{
		preference.themeKey(): string(themes), preference.localeKey(): preference.localeChoice, preference.accentKey(): preference.accent,
	}, Removed: []string{}}, false)
	preference.applicationEventPublisher.PublishEvent(StorageWritten{Area: "session", Values: map[string]string{
		preference.interfaceFontKey(): preference.interfaceFont, preference.composerFontKey(): preference.composerFont, preference.mailKey(): string(mail),
	}, Removed: []string{}}, false)
}

// apply publishes every <html> attribute that the preferences decide.
func (preference *PreferenceService) apply() {
	resolved := AppearanceUtils{}.Resolved(preference.themes, preference.systemDark)
	locale := LocaleUtils{}.Resolve(preference.localeChoice, preference.language)
	attributes := AppearanceUtils{}.Merged(AppearanceUtils{}.MailAttributes(preference.mail), AppearanceUtils{}.ThemeAttributes(preference.themes, resolved))
	attributes["data-accent"] = preference.accent
	attributes["data-interface-font"] = preference.interfaceFont
	attributes["data-composer-font"] = preference.composerFont
	attributes["lang"] = locale
	attributes["dir"] = LocaleUtils{}.Direction(locale)
	attributes["data-interface-locale"] = locale
	attributes["data-interface-locale-choice"] = preference.localeChoice
	preference.ThemeToggleLabel = "Switch to " + AppearanceUtils{}.Opposite(AppearanceUtils{}.BaseMode(resolved)) + " theme"
	preference.applicationEventPublisher.PublishEvent(DocumentAttributesChanged{Owner: "preferences", Attributes: attributes}, false)
}

func (preference *PreferenceService) chosenThemes(chosen *ThemePreferences) ThemePreferences {
	if chosen == nil {
		return preference.themes
	}
	return AppearanceUtils{}.Themes(*chosen)
}

func (preference *PreferenceService) chosenMail(chosen *MailPreferences) MailPreferences {
	if chosen == nil {
		return preference.mail
	}
	return AppearanceUtils{}.ChosenMail(chosen)
}


// keys are the legacy storage names of the shared preference scripts.
func (PreferenceService) themeKey() string { return "caduceus-theme-preferences" }

func (PreferenceService) localeKey() string { return "caduceus-interface-locale-v1" }

func (PreferenceService) accentKey() string { return "caduceus-accent-color" }

func (PreferenceService) interfaceFontKey() string { return "caduceus_interface_font" }

func (PreferenceService) composerFontKey() string { return "caduceus_composer_font" }

func (PreferenceService) mailKey() string { return "caduceus_mail_preferences" }
