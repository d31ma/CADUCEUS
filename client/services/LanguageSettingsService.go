// LanguageSettingsService is the interface language choice. It applies the
// choice at once, saves it, and puts the previous choice back if saving fails.
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
	service.Choice = LocaleUtils{}.Normalize(nil)
}

// @OnEvent({"replay":true})
func (service *LanguageSettingsService) Open(session AccountSession) {
	service.authorization = "Bearer " + session.Token
}

// @OnEvent({"replay":true})
func (service *LanguageSettingsService) Hear(changed PreferencesChanged) {
	service.language = changed.Language
}

// @OnEvent({"replay":true})
func (service *LanguageSettingsService) Load(preferences UserPreferences) {
	service.Choice = LocaleUtils{}.Normalize(preferences.Values[service.key()])
}

// T answers a message of the language section in the shown locale.
func (service *LanguageSettingsService) T(key string) string {
	return LocaleUtils{}.Translate(key, LocaleUtils{}.Resolve(service.Choice, service.language))
}

func (service *LanguageSettingsService) Options() []LocaleOption {
	return LocaleUtils{}.Options(LocaleUtils{}.Resolve(service.Choice, service.language))
}

func (service *LanguageSettingsService) Select(value string) error {
	FlowUtils{}.When(!service.Saving, func() { service.save(service.Choice, LocaleUtils{}.Normalize(value)) })
	return nil
}

func (service *LanguageSettingsService) save(previous string, next string) {
	service.apply(next)
	service.Saving = true
	service.Error = ""
	stored, failure := service.usersClient.SavePreferences(service.authorization, map[string]any{service.key(): next})
	service.Saving = false
	service.showSaved(previous, PreferenceUtils{}.Object(stored), failure)
}

func (service *LanguageSettingsService) showSaved(previous string, stored map[string]any, failure error) {
	if failure != nil {
		service.apply(previous)
		service.Error = service.T("locale.saveError")
		return
	}
	service.apply(LocaleUtils{}.Normalize(stored[service.key()]))
	service.applicationEventPublisher.PublishEvent(ToastRequested{Message: service.T("locale.saved")}, false)
}

func (service *LanguageSettingsService) apply(choice string) {
	service.Choice = choice
	service.applicationEventPublisher.PublishEvent(LocalePreference{Choice: choice}, false)
}

func (service *LanguageSettingsService) key() string { return "interfaceLocale" }
