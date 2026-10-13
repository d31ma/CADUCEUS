// SettingsServicesTest covers the settings services' state that needs no
// server and publishes nothing.
// @Test(SettingsTestSetup, {"partition":"tests"})
type SettingsServicesTest struct {
	behaviors *BehaviorContext
}

func (test *SettingsServicesTest) TestTheViewLoadsUntilEverySectionAnswers() {
	settings := &SettingsService{}
	settings.Start()
	Assert.IsTrue(settings.Loading)
	settings.Track(SettingsActivity{Section: "aliases"})
	settings.Track(SettingsActivity{Section: "preferences"})
	Assert.IsTrue(settings.Loading)
}

func (test *SettingsServicesTest) TestTranslationChoicesComeFromTheReadersKey() {
	translation := &TranslationPreferencesService{}
	translation.Start()
	Assert.Equals(0, len(translation.AutomaticRows))
	translation.HearStorage(StoredValues{Local: map[string]string{"caduceus-translation-preferences-v1": `{"automatic":["es"],"disabled":["fr","es"]}`}})
	Assert.Equals(0, len(translation.AutomaticRows))
	Assert.Equals([]LanguageOption{{ID: "fr", Label: "French"}, {ID: "es", Label: "Spanish"}}, translation.DisabledRows)
}
