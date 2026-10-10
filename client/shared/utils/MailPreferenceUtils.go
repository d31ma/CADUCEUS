import "strings"

// MailPreferenceUtils is the Go form of shared/scripts/mail-preferences.js:
// the defaults, the closed catalogs and the normalization of one profile.
type MailPreferenceUtils struct{}

func (utils MailPreferenceUtils) Defaults() MailPreferences {
	return MailPreferences{
		WorkspaceLayout: utils.layouts()[0], InterfaceScale: 1, MarkAsReadDelay: utils.readDelays()[1],
		BackspaceAction: utils.backspaceActions()[0], SwipeEnabled: true, ConversationOrder: utils.orders()[0],
		EmailRenderMode: utils.renderModes()[0], RichText: true, Spellcheck: true,
		DefaultSendAction: utils.sendActions()[0], DefaultReplyAction: utils.replyActions()[0], UndoSendDelay: utils.undoDelays()[1],
		AutoCcBySender: map[string][]string{}, AutoBccBySender: map[string][]string{},
		DisplayAttachmentPreviews: true, UnsnoozeToTop: true, FindInMailDisabledDomains: []string{},
	}
}

// Normalize reads one stored or submitted profile. Values outside a catalog
// fall back to the default, as normalizeMailPreferences does.
func (utils MailPreferenceUtils) Normalize(value any) MailPreferences {
	defaults := utils.Defaults()
	preferences := utils.Defaults()
	PreferenceUtils{}.Decode(PreferenceUtils{}.Object(value), &preferences)
	preferences.WorkspaceLayout = PreferenceUtils{}.Choice(preferences.WorkspaceLayout, utils.layouts(), defaults.WorkspaceLayout)
	preferences.BackspaceAction = PreferenceUtils{}.Choice(preferences.BackspaceAction, utils.backspaceActions(), defaults.BackspaceAction)
	preferences.ConversationOrder = PreferenceUtils{}.Choice(preferences.ConversationOrder, utils.orders(), defaults.ConversationOrder)
	preferences.EmailRenderMode = PreferenceUtils{}.Choice(preferences.EmailRenderMode, utils.renderModes(), defaults.EmailRenderMode)
	preferences.SpellcheckLanguage = PreferenceUtils{}.Choice(preferences.SpellcheckLanguage, utils.spellcheckIds(), defaults.SpellcheckLanguage)
	preferences.DefaultSendAction = PreferenceUtils{}.Choice(preferences.DefaultSendAction, utils.sendActions(), defaults.DefaultSendAction)
	preferences.DefaultReplyAction = PreferenceUtils{}.Choice(preferences.DefaultReplyAction, utils.replyActions(), defaults.DefaultReplyAction)
	preferences.InterfaceScale = PreferenceUtils{}.Number(preferences.InterfaceScale, []float64{0.8, 0.9, 1, 1.1, 1.2}, defaults.InterfaceScale)
	preferences.MarkAsReadDelay = PreferenceUtils{}.Number(preferences.MarkAsReadDelay, utils.readDelays(), defaults.MarkAsReadDelay)
	preferences.UndoSendDelay = PreferenceUtils{}.Number(preferences.UndoSendDelay, utils.undoDelays(), defaults.UndoSendDelay)
	preferences.AutoCcBySender = utils.recipientMap(preferences.AutoCcBySender)
	preferences.AutoBccBySender = utils.recipientMap(preferences.AutoBccBySender)
	preferences.DefaultFrom = PreferenceUtils{}.Lower(preferences.DefaultFrom)
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

func (MailPreferenceUtils) layouts() []string { return []string{"split", "list", "split-vertical"} }

func (MailPreferenceUtils) readDelays() []float64 { return []float64{0, 500, 2000, -1} }

func (MailPreferenceUtils) undoDelays() []float64 { return []float64{0, 5000, 15000, 30000, 60000} }

func (MailPreferenceUtils) backspaceActions() []string { return []string{"archive", "trash"} }

func (MailPreferenceUtils) orders() []string { return []string{"ascending", "descending"} }

func (MailPreferenceUtils) renderModes() []string { return []string{"theme", "light", "dark"} }

func (MailPreferenceUtils) sendActions() []string { return []string{"send", "send-and-archive"} }

func (MailPreferenceUtils) replyActions() []string { return []string{"reply", "reply-all"} }

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
