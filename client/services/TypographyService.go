// TypographyService chooses the local interface and composer fonts. The
// preview applies at once; a failed save restores both stored choices.
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
	service.read(map[string]any{})
}

// @OnEvent({"replay":true})
func (service *TypographyService) Open(session AccountSession) {
	service.authorization = "Bearer " + session.Token
}

// @OnEvent({"replay":true})
func (service *TypographyService) Load(preferences UserPreferences) {
	service.read(preferences.Values)
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
	service.save(map[string]any{service.interfaceKey(): FontPreferenceUtils{}.NormalizeInterface(value)})
	return nil
}

func (service *TypographyService) SetComposerFont(value string) error {
	service.save(map[string]any{service.composerKey(): FontPreferenceUtils{}.NormalizeComposer(value)})
	return nil
}

func (service *TypographyService) save(patch map[string]any) {
	FlowUtils{}.When(!service.Saving, func() { service.send(patch) })
}

func (service *TypographyService) send(patch map[string]any) {
	previous := service.current()
	service.apply(service.merged(patch))
	service.Error = ""
	service.Saving = true
	stored, failure := service.usersClient.SavePreferences(service.authorization, patch)
	service.Saving = false
	FlowUtils{}.Choose(failure == nil, func() {
		service.apply(PreferenceUtils{}.Object(stored))
		service.applicationEventPublisher.PublishEvent(ToastRequested{Message: "Typography preference saved."}, false)
	}, func() {
		service.apply(previous)
		service.Error = "Could not save typography preferences."
	})
}

func (service *TypographyService) current() map[string]any {
	return map[string]any{service.interfaceKey(): service.InterfaceFont, service.composerKey(): service.ComposerFont}
}

func (service *TypographyService) merged(patch map[string]any) map[string]any {
	merged := service.current()
	for key, value := range patch {
		merged[key] = value
	}
	return merged
}

func (service *TypographyService) read(values map[string]any) {
	service.InterfaceFont = FontPreferenceUtils{}.NormalizeInterface(values[service.interfaceKey()])
	service.ComposerFont = FontPreferenceUtils{}.NormalizeComposer(values[service.composerKey()])
}

func (service *TypographyService) apply(values map[string]any) {
	service.read(values)
	service.applicationEventPublisher.PublishEvent(FontPreferences{InterfaceFont: service.InterfaceFont, ComposerFont: service.ComposerFont}, false)
}

func (service *TypographyService) interfaceKey() string { return "interfaceFont" }

func (service *TypographyService) composerKey() string { return "composerFont" }
