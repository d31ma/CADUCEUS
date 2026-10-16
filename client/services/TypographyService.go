// TypographyService chooses the local interface and composer fonts. The
// preview applies at once through PreferenceService; a failed save restores
// both stored choices.
// @Service
type TypographyService struct {
	// @Needs
	usersClient               *UsersClient
	applicationEventPublisher ApplicationEventPublisher
	authorization             string
	InterfaceFont             string
	ComposerFont              string
	Saving                    bool
	Error                     string
}

// @OnStart
func (service *TypographyService) Start() {
	service.read(UserPreferences{})
}

// @OnEvent({"replay":true})
func (service *TypographyService) HearSession(changed SessionChanged) {
	service.authorization = "Bearer " + changed.Token
}

// @OnEvent({"replay":true})
func (service *TypographyService) Load(preferences UserPreferences) {
	service.read(preferences)
}

func (service *TypographyService) InterfaceOptions() []FontOption {
	return FontPreferenceUtils{}.InterfaceOptions()
}

func (service *TypographyService) ComposerOptions() []FontOption {
	return FontPreferenceUtils{}.ComposerOptions()
}

func (service *TypographyService) InterfacePreview() string {
	return FontPreferenceUtils{}.InterfacePreview(service.InterfaceFont)
}

func (service *TypographyService) ComposerPreview() string {
	return FontPreferenceUtils{}.ComposerPreview(service.ComposerFont)
}

func (service *TypographyService) SetInterfaceFont(value string) error {
	service.save(map[string]any{"interfaceFont": FontPreferenceUtils{}.NormalizeInterface(value)})
	return nil
}

func (service *TypographyService) SetComposerFont(value string) error {
	service.save(map[string]any{"composerFont": FontPreferenceUtils{}.NormalizeComposer(value)})
	return nil
}

func (service *TypographyService) save(patch map[string]any) {
	FlowUtils{}.When(!service.Saving, func() { service.send(patch) })()
}

func (service *TypographyService) send(patch map[string]any) {
	previous := service.current()
	next := previous
	PreferenceUtils{}.Decode(patch, &next)
	service.apply(next)
	service.Error = ""
	service.Saving = true
	stored, failure := service.usersClient.SavePreferences(service.authorization, patch)
	service.Saving = false
	FlowUtils{}.Either(failure == nil, func() { service.saved(stored) }, func() {
		service.apply(previous)
		service.Error = HTTPUtils{}.Message(failure, "Could not save typography preferences.", "Network error.")
	})()
}

func (service *TypographyService) saved(stored UserPreferences) {
	service.apply(stored)
	service.applicationEventPublisher.PublishEvent(ToastRequested{Message: "Typography preference saved."}, false)
}

func (service *TypographyService) current() UserPreferences {
	return UserPreferences{InterfaceFont: service.InterfaceFont, ComposerFont: service.ComposerFont}
}

func (service *TypographyService) read(preferences UserPreferences) {
	service.InterfaceFont = FontPreferenceUtils{}.NormalizeInterface(preferences.InterfaceFont)
	service.ComposerFont = FontPreferenceUtils{}.NormalizeComposer(preferences.ComposerFont)
}

func (service *TypographyService) apply(preferences UserPreferences) {
	service.read(preferences)
	service.applicationEventPublisher.PublishEvent(PreferencesChosen{InterfaceFont: service.InterfaceFont, ComposerFont: service.ComposerFont}, false)
}
