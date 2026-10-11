// SettingsUtilsTest covers the pure utilities of the settings sections:
// translation choices, mail preferences, shortcuts, locales and mailboxes.
// @Test(SettingsTestSetup, {"partition":"tests"})
type SettingsUtilsTest struct {
	behaviors *BehaviorContext
}

func (test *SettingsUtilsTest) TestStoredTranslationChoicesKeepPrimaryLanguages() {
	choices := TranslationLanguageUtils{}.Stored(`{"automatic":["fr-CA","FR","de"],"disabled":["de-AT"]}`)
	Assert.Equals([]string{"fr"}, choices.Automatic)
	Assert.Equals([]string{"de"}, choices.Disabled)
	Assert.Equals(TranslationChoices{Automatic: []string{}, Disabled: []string{}}, TranslationLanguageUtils{}.Stored("not json"))
}

func (test *SettingsUtilsTest) TestRestoringALanguageForgetsIt() {
	choices := TranslationLanguageUtils{}.Without(TranslationChoices{Automatic: []string{"fr", "es"}, Disabled: []string{"de"}}, "fr")
	Assert.Equals(`{"automatic":["es"],"disabled":["de"]}`, TranslationLanguageUtils{}.Encoded(choices))
	Assert.Equals("caduceus-translation-preferences-v1", TranslationLanguageUtils{}.Key())
}

func (test *SettingsUtilsTest) TestTranslationRowsNameTheLanguages() {
	rows := TranslationLanguageUtils{}.Rows([]string{"fr", "xx"})
	Assert.Equals(LanguageOption{ID: "fr", Label: "French"}, rows[0])
	Assert.Equals(LanguageOption{ID: "xx", Label: "xx"}, rows[1])
}

// @ValueSource([0, 500, 2000, -1])
func (test *SettingsUtilsTest) TestMarkAsReadDelayKeepsACatalogValue(delay int) {
	Assert.Equals(delay, MailPreferenceUtils{}.Normalize(&MailPreferences{MarkAsReadDelay: delay}).MarkAsReadDelay)
}

// @ValueSource([7, 1000, -5])
func (test *SettingsUtilsTest) TestUndoSendDelayFallsBackOutsideTheCatalog(delay int) {
	Assert.Equals(MailPreferenceUtils{}.Defaults().UndoSendDelay, MailPreferenceUtils{}.Normalize(&MailPreferences{UndoSendDelay: delay}).UndoSendDelay)
}

func (test *SettingsUtilsTest) TestMailPreferencesReadAnUnknownSpellcheckLanguage() {
	preferences := MailPreferenceUtils{}.Normalize(&MailPreferences{SpellcheckLanguage: "klingon"})
	Assert.Equals(MailPreferenceUtils{}.Defaults().SpellcheckLanguage, preferences.SpellcheckLanguage)
	Assert.Equals(map[string][]string{}, preferences.AutoCcBySender)
	Assert.Equals(LanguageOption{ID: "", Label: "Device default"}, MailPreferenceUtils{}.SpellcheckLanguages()[0])
}

func (test *SettingsUtilsTest) TestRecipientsSplitTheField() {
	Assert.Equals([]string{"ada@x.com", "bob@x.com"}, MailPreferenceUtils{}.Recipients(" ada@x.com, bob@x.com ,,ada@x.com"))
}

func (test *SettingsUtilsTest) TestChoicesAnswerConditions() {
	Assert.Equals("on", ChoiceUtils{}.Text(true, "on", "off"))
	Assert.Equals("off", ChoiceUtils{}.Text(false, "on", "off"))
	Assert.IsTrue(ChoiceUtils{}.All(true, true))
	Assert.Equals(false, ChoiceUtils{}.All(true, false))
	Assert.IsTrue(ChoiceUtils{}.Any(false, true))
	Assert.Equals(false, ChoiceUtils{}.Any())
}

func (test *SettingsUtilsTest) TestTimestampLabelsUseTheOffset() {
	Assert.Equals("Oct 10, 2026", DateLabelUtils{}.StoredDate("2026-10-10T23:30:00Z", 0))
	Assert.Equals("Oct 11, 2026", DateLabelUtils{}.StoredDate("2026-10-10T23:30:00Z", 60))
	Assert.Equals("", DateLabelUtils{}.StoredDateTime("yesterday", 0))
}

func (test *SettingsUtilsTest) TestShortcutValidationFindsConflicts() {
	catalog := []KeymapAction{{ID: "compose", Label: "Compose", Group: "Mail", Scope: "global", Binding: "c"}, {ID: "reply", Label: "Reply", Group: "Mail", Scope: "global", Binding: "r"}}
	Assert.IsTrue(KeyboardShortcutUtils{}.Validate(catalog, map[string]string{"compose": "Ctrl + K"}, false).Valid)
	Assert.Equals(map[string]string{"compose": "ctrl+k"}, KeyboardShortcutUtils{}.Validate(catalog, map[string]string{"compose": "Ctrl + K"}, false).Overrides)
	Assert.Equals(false, KeyboardShortcutUtils{}.Validate(catalog, map[string]string{"compose": "r"}, false).Valid)
	Assert.Equals(false, KeyboardShortcutUtils{}.Validate(catalog, map[string]string{"missing": "x"}, false).Valid)
}

func (test *SettingsUtilsTest) TestShortcutGroupsFollowTheCatalog() {
	catalog := []KeymapAction{{ID: "compose", Label: "Compose", Group: "Mail", Scope: "global", Binding: "c"}, {ID: "search", Label: "Search", Group: "Navigation", Scope: "global", Binding: "/"}}
	groups := KeyboardShortcutUtils{}.Groups(catalog, map[string]string{}, false)
	Assert.Equals(2, len(groups))
	Assert.Equals("Mail", groups[0].Label)
}

func (test *SettingsUtilsTest) TestLocaleOptionsNameTheAutomaticChoice() {
	options := LocaleUtils{}.Options("en")
	Assert.Equals("auto", options[0].ID)
	Assert.IsTrue(options[0].Label != "")
	Assert.Equals(LocaleOption{ID: "de", Label: "Deutsch"}, options[2])
}

func (test *SettingsUtilsTest) TestMailboxesAreHostedOnce() {
	Assert.Equals([]string{"a.com", "b.com"}, MailboxUtils{}.Hosted([]string{"B.com a.com", "a.com"}))
	Assert.Equals("Work", MailboxUtils{}.Label(map[string]string{"a.com": "Work"}, " A.com "))
	Assert.Equals("b.com", MailboxUtils{}.Label(map[string]string{}, "b.com"))
}
