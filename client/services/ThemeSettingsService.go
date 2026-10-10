// ThemeSettingsService chooses the interface theme, or the light and dark
// themes that "Automatic" follows. A choice applies at once and is undone
// when saving fails.
// @Service
type ThemeSettingsService struct {
	Error  string
	Saving bool
	// @Needs
	usersClient               *UsersClient
	applicationEventPublisher ApplicationEventPublisher
	authorization             string
	Preferences               ThemePreferences
}

// @OnStart
func (service *ThemeSettingsService) Start() {
	service.Preferences = ThemePreferenceUtils{}.Normalize(nil)
}

// @OnEvent({"replay":true})
func (service *ThemeSettingsService) Open(session AccountSession) {
	service.authorization = "Bearer " + session.Token
}

// @OnEvent({"replay":true})
func (service *ThemeSettingsService) Load(preferences UserPreferences) {
	service.Preferences = ThemePreferenceUtils{}.Normalize(preferences.Values[service.key()])
}

func (service *ThemeSettingsService) Select(theme string) error {
	service.save(map[string]any{"activeTheme": theme})
	return nil
}

// SelectAutomatic sets the theme that "Automatic" uses in light or dark mode.
func (service *ThemeSettingsService) SelectAutomatic(slot string, theme string) error {
	service.save(map[string]any{slot + "Theme": theme})
	return nil
}

func (service *ThemeSettingsService) Options() []ThemeOption {
	return ThemePreferenceUtils{}.Options()
}

func (service *ThemeSettingsService) LightOptions() []ThemeOption {
	return ThemePreferenceUtils{}.LightOptions()
}

func (service *ThemeSettingsService) DarkOptions() []ThemeOption {
	return ThemePreferenceUtils{}.DarkOptions()
}

func (service *ThemeSettingsService) PreviewStyle(theme ThemeOption) string {
	return ThemePreferenceUtils{}.PreviewStyle(theme)
}

func (service *ThemeSettingsService) save(patch map[string]any) {
	FlowUtils{}.When(!service.Saving, func() { service.send(service.Preferences, service.merged(patch)) })
}

func (service *ThemeSettingsService) send(previous ThemePreferences, next ThemePreferences) {
	service.apply(next)
	service.Saving = true
	service.Error = ""
	stored, failure := service.usersClient.SavePreferences(service.authorization, map[string]any{service.key(): next})
	service.Saving = false
	FlowUtils{}.Choose(failure == nil, func() {
		service.apply(ThemePreferenceUtils{}.Normalize(PreferenceUtils{}.Object(stored)[service.key()]))
		service.applicationEventPublisher.PublishEvent(ToastRequested{Message: "Theme preference saved."}, false)
	}, func() {
		service.apply(previous)
		service.Error = "Could not save theme."
	})
}

func (service *ThemeSettingsService) merged(patch map[string]any) ThemePreferences {
	values := map[string]any{}
	PreferenceUtils{}.Decode(service.Preferences, &values)
	for key, value := range patch {
		values[key] = value
	}
	return ThemePreferenceUtils{}.Normalize(values)
}

func (service *ThemeSettingsService) apply(preferences ThemePreferences) {
	service.Preferences = preferences
	service.applicationEventPublisher.PublishEvent(preferences, false)
}

func (service *ThemeSettingsService) key() string { return "themePreferences" }
