// AccentColorService offers the accent presets. The application shell keeps
// and applies the chosen accent; this service hears its choice and asks for
// a new one with AccentColorChoice.
// @Service
type AccentColorService struct {
	applicationEventPublisher ApplicationEventPublisher
	Current                   string
}

// @OnStart
func (service *AccentColorService) Start() {
	service.Current = AccentColorUtils{}.Presets()[0].Key
}

// @OnEvent({"replay":true})
func (service *AccentColorService) Hear(choice AccentColorChoice) {
	FlowUtils{}.When(AccentColorUtils{}.Known(choice.Key), func() { service.Current = choice.Key })
}

func (service *AccentColorService) Presets() []AccentPreset {
	return AccentColorUtils{}.Presets()
}

func (service *AccentColorService) CurrentLabel() string {
	return AccentColorUtils{}.Label(service.Current)
}

func (service *AccentColorService) Select(key string) {
	FlowUtils{}.When(AccentColorUtils{}.Known(key), func() {
		service.Current = key
		service.applicationEventPublisher.PublishEvent(AccentColorChoice{Key: key}, true)
	})
}
