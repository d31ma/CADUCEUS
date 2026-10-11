// LanguageSettingsService is the interface language choice. It applies the
// choice at once through PreferenceService, saves it, and puts the previous
// choice back if saving fails.
// @Service
type LanguageSettingsService struct {
	// @Needs
	usersClient               *UsersClient
	applicationEventPublisher ApplicationEventPublisher
	authorization             string
	language                  string
	Choice                    string
	Saving                    bool
	Error                     string
}

// @OnStart
func (service *LanguageSettingsService) Start() {
	service.Choice = LocaleUtils{}.Choice("")
}

// @OnEvent({"replay":true})
func (service *LanguageSettingsService) HearSession(changed SessionChanged) {
	service.authorization = "Bearer " + changed.Token
}

// @OnEvent({"replay":true})
func (service *LanguageSettingsService) Hear(changed PreferencesChanged) {
	service.language = changed.Language
}

// @OnEvent({"replay":true})
func (service *LanguageSettingsService) Load(preferences UserPreferences) {
	service.Choice = LocaleUtils{}.Choice(preferences.InterfaceLocale)
}

// T answers a message of the language section in the shown locale.
func (service *LanguageSettingsService) T(key string) string {
	return LocaleUtils{}.Translate(service.locale(), key, map[string]string{})
}

func (service *LanguageSettingsService) Options() []LocaleOption {
	return LocaleUtils{}.Options(service.locale())
}

func (service *LanguageSettingsService) Select(value string) error {
	FlowUtils{}.When(!service.Saving, func() { service.save(service.Choice, LocaleUtils{}.Choice(value)) })()
	return nil
}

func (service *LanguageSettingsService) save(previous string, next string) {
	service.apply(next)
	service.Saving = true
	service.Error = ""
	stored, failure := service.usersClient.SavePreferences(service.authorization, map[string]any{"interfaceLocale": next})
	service.Saving = false
	FlowUtils{}.Either(failure == nil, func() { service.saved(stored) }, func() { service.failed(previous) })()
}

func (service *LanguageSettingsService) saved(stored UserPreferences) {
	service.apply(LocaleUtils{}.Choice(stored.InterfaceLocale))
	service.applicationEventPublisher.PublishEvent(ToastRequested{Message: service.T("locale.saved")}, false)
}

func (service *LanguageSettingsService) failed(previous string) {
	service.apply(previous)
	service.Error = service.T("locale.saveError")
}

func (service *LanguageSettingsService) apply(choice string) {
	service.Choice = choice
	service.applicationEventPublisher.PublishEvent(PreferencesChosen{InterfaceLocale: choice}, false)
}

func (service *LanguageSettingsService) locale() string {
	return LocaleUtils{}.Resolve(service.Choice, service.language)
}
