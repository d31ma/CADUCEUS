// ThemeSettingsService chooses the interface theme, or the light and dark
// themes that "Automatic" follows. A choice applies at once through
// PreferenceService and is undone when saving fails.
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
func (service *ThemeSettingsService) HearSession(changed SessionChanged) {
	service.authorization = "Bearer " + changed.Token
}

// @OnEvent({"replay":true})
func (service *ThemeSettingsService) Load(preferences UserPreferences) {
	service.Preferences = ThemePreferenceUtils{}.Normalize(preferences.ThemePreferences)
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
	FlowUtils{}.When(!service.Saving, func() { service.send(service.Preferences, service.merged(patch)) })()
}

func (service *ThemeSettingsService) send(previous ThemePreferences, next ThemePreferences) {
	service.apply(next)
	service.Saving = true
	service.Error = ""
	stored, failure := service.usersClient.SavePreferences(service.authorization, map[string]any{"themePreferences": next})
	service.Saving = false
	FlowUtils{}.Either(failure == nil, func() { service.saved(stored) }, func() {
		service.apply(previous)
		service.Error = HTTPUtils{}.Message(failure, "Could not save theme.", "Network error.")
	})()
}

func (service *ThemeSettingsService) saved(stored UserPreferences) {
	service.apply(ThemePreferenceUtils{}.Normalize(stored.ThemePreferences))
	service.applicationEventPublisher.PublishEvent(ToastRequested{Message: "Theme preference saved."}, false)
}

func (service *ThemeSettingsService) merged(patch map[string]any) ThemePreferences {
	next := service.Preferences
	PreferenceUtils{}.Decode(patch, &next)
	return ThemePreferenceUtils{}.Normalize(&next)
}

func (service *ThemeSettingsService) apply(preferences ThemePreferences) {
	service.Preferences = preferences
	chosen := preferences
	service.applicationEventPublisher.PublishEvent(PreferencesChosen{ThemePreferences: &chosen}, false)
}
