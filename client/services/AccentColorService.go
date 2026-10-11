// AccentColorService offers the accent presets. PreferenceService keeps the
// chosen accent on this device and applies it to the document; this service
// reads the stored choice and asks for a new one with PreferencesChosen.
// @Service
type AccentColorService struct {
	applicationEventPublisher ApplicationEventPublisher
	Current                   string
}

// @OnStart
func (service *AccentColorService) Start() {
	service.Current = AppearanceUtils{}.Accent("")
}

// @OnEvent({"replay":true})
func (service *AccentColorService) HearStorage(stored StoredValues) {
	service.Current = AppearanceUtils{}.Accent(stored.Local["caduceus-accent-color"])
}

func (service *AccentColorService) Presets() []AccentPreset {
	return AccentColorUtils{}.Presets()
}

func (service *AccentColorService) CurrentLabel() string {
	return AccentColorUtils{}.Label(service.Current)
}

func (service *AccentColorService) Select(key string) {
	FlowUtils{}.When(AccentColorUtils{}.Known(key), func() { service.choose(key) })()
}

func (service *AccentColorService) choose(key string) {
	service.Current = key
	service.applicationEventPublisher.PublishEvent(PreferencesChosen{AccentColor: key}, false)
}
