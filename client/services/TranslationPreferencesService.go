// TranslationPreferencesService shows and resets the per-language
// translation choices the message reader keeps on this device. They live
// under the reader's own local storage key, so the service reads them from
// StoredValues and writes them back with StorageWritten.
// @Service
type TranslationPreferencesService struct {
	applicationEventPublisher ApplicationEventPublisher
	choices                   TranslationChoices
	AutomaticRows             []LanguageOption
	DisabledRows              []LanguageOption
}

// @OnStart
func (service *TranslationPreferencesService) Start() {
	service.show(TranslationChoices{})
}

// @OnEvent({"replay":true})
func (service *TranslationPreferencesService) HearStorage(stored StoredValues) {
	service.show(TranslationLanguageUtils{}.Stored(stored.Local[TranslationLanguageUtils{}.Key()]))
}

// Restore asks again before translating one language.
func (service *TranslationPreferencesService) Restore(language string) {
	next := TranslationLanguageUtils{}.Without(service.choices, language)
	service.applicationEventPublisher.PublishEvent(StorageWritten{Area: service.area(), Values: map[string]string{TranslationLanguageUtils{}.Key(): TranslationLanguageUtils{}.Encoded(next)}, Removed: []string{}}, false)
	service.show(next)
}

// ResetAll forgets every translation choice.
func (service *TranslationPreferencesService) ResetAll() {
	service.applicationEventPublisher.PublishEvent(StorageWritten{Area: service.area(), Values: map[string]string{}, Removed: []string{TranslationLanguageUtils{}.Key()}}, false)
	service.show(TranslationChoices{})
	service.applicationEventPublisher.PublishEvent(ToastRequested{Message: "Translation preferences reset."}, false)
}

func (service *TranslationPreferencesService) show(choices TranslationChoices) {
	service.choices = choices
	service.AutomaticRows = TranslationLanguageUtils{}.Rows(choices.Automatic)
	service.DisabledRows = TranslationLanguageUtils{}.Rows(choices.Disabled)
}

func (service *TranslationPreferencesService) area() string { return "local" }
