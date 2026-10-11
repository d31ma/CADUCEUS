import (
	"slices"
	"strings"
)

// MailPreferenceUtils is the settings side of the mail-client preferences:
// AppearanceUtils keeps the layout catalogs that the whole client shares,
// and this class adds the catalogs that only the settings form offers, such
// as the delays and the spellcheck languages.
type MailPreferenceUtils struct{}

// Defaults is the mail-client profile before the user changes it.
func (MailPreferenceUtils) Defaults() MailPreferences {
	return AppearanceUtils{}.DefaultMail()
}

// Normalize reads one stored or submitted profile. Values outside a catalog
// fall back to the default, as normalizeMailPreferences does.
func (utils MailPreferenceUtils) Normalize(chosen *MailPreferences) MailPreferences {
	defaults := utils.Defaults()
	preferences := AppearanceUtils{}.ChosenMail(chosen)
	preferences.SpellcheckLanguage = PreferenceUtils{}.Choice(preferences.SpellcheckLanguage, utils.spellcheckIds(), defaults.SpellcheckLanguage)
	preferences.MarkAsReadDelay = utils.delay(preferences.MarkAsReadDelay, utils.readDelays(), defaults.MarkAsReadDelay)
	preferences.UndoSendDelay = utils.delay(preferences.UndoSendDelay, utils.undoDelays(), defaults.UndoSendDelay)
	preferences.AutoCcBySender = utils.recipientMap(preferences.AutoCcBySender)
	preferences.AutoBccBySender = utils.recipientMap(preferences.AutoBccBySender)
	preferences.FindInMailDisabledDomains = PreferenceUtils{}.Unique(preferences.FindInMailDisabledDomains)
	return preferences
}

// Recipients turns one comma-separated field into the stored address list.
func (MailPreferenceUtils) Recipients(value string) []string {
	return PreferenceUtils{}.Unique(strings.Split(value, ","))
}

func (utils MailPreferenceUtils) SpellcheckLanguages() []LanguageOption {
	return utils.languages()
}

func (utils MailPreferenceUtils) languages() []LanguageOption {
	options := []LanguageOption{}
	for _, pair := range utils.spellcheckPairs() {
		options = append(options, LanguageOption{ID: pair[0], Label: pair[1]})
	}
	return options
}

func (MailPreferenceUtils) recipientMap(recipients map[string][]string) map[string][]string {
	if recipients == nil {
		return map[string][]string{}
	}
	return recipients
}

func (MailPreferenceUtils) delay(value int, allowed []int, fallback int) int {
	if slices.Contains(allowed, value) {
		return value
	}
	return fallback
}

func (MailPreferenceUtils) readDelays() []int { return []int{0, 500, 2000, -1} }

func (MailPreferenceUtils) undoDelays() []int { return []int{0, 5000, 15000, 30000, 60000} }

func (utils MailPreferenceUtils) spellcheckIds() []string {
	ids := []string{}
	for _, pair := range utils.spellcheckPairs() {
		ids = append(ids, pair[0])
	}
	return ids
}

func (MailPreferenceUtils) spellcheckPairs() [][]string {
	return [][]string{
		{"", "Device default"}, {"af", "Afrikaans"}, {"bg", "Bulgarian"}, {"ca", "Catalan"},
		{"cs", "Czech"}, {"da", "Danish"}, {"de", "German"}, {"el", "Greek"},
		{"en-AU", "English (Australia)"}, {"en-CA", "English (Canada)"}, {"en-GB", "English (UK)"}, {"en-US", "English (US)"},
		{"es", "Spanish"}, {"et", "Estonian"}, {"fo", "Faroese"}, {"fr", "French"},
		{"he", "Hebrew"}, {"hi", "Hindi"}, {"hr", "Croatian"}, {"hu", "Hungarian"},
		{"it", "Italian"}, {"ko", "Korean"}, {"lt", "Lithuanian"}, {"lv", "Latvian"},
		{"nb", "Norwegian Bokmal"}, {"nl", "Dutch"}, {"pl", "Polish"}, {"pt", "Portuguese"},
		{"pt-BR", "Portuguese (Brazil)"}, {"ro", "Romanian"}, {"ru", "Russian"}, {"sk", "Slovak"},
		{"sl", "Slovenian"}, {"sq", "Albanian"}, {"sr", "Serbian"}, {"sv", "Swedish"},
		{"tr", "Turkish"}, {"uk", "Ukrainian"}, {"vi", "Vietnamese"},
	}
}
