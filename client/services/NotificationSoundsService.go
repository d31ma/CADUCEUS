import "maps"

// NotificationSoundsService chooses the locally generated cue for each mail
// event and previews it. SoundPreviewService.js plays the cue.
// @Service
type NotificationSoundsService struct {
	// @Needs
	usersClient               *UsersClient
	applicationEventPublisher ApplicationEventPublisher
	authorization             string
	Sounds                    map[string]string
	Saving                    bool
	Error                     string
}

// @OnStart
func (service *NotificationSoundsService) Start() {
	service.Sounds = NotificationSoundUtils{}.Normalize(nil)
}

// @OnEvent({"replay":true})
func (service *NotificationSoundsService) HearSession(changed SessionChanged) {
	service.authorization = "Bearer " + changed.Token
}

// @OnEvent({"replay":true})
func (service *NotificationSoundsService) Load(preferences UserPreferences) {
	service.Sounds = NotificationSoundUtils{}.Normalize(preferences.NotificationSounds)
}

func (service *NotificationSoundsService) Events() []SoundEvent {
	return NotificationSoundUtils{}.Events()
}

func (service *NotificationSoundsService) Options() []SoundOption {
	return NotificationSoundUtils{}.Options()
}

// Sound is the chosen cue of one event, or "none" for an unknown event.
func (service *NotificationSoundsService) Sound(event string) string {
	return ChoiceUtils{}.Text(service.Sounds[event] != "", service.Sounds[event], NotificationSoundUtils{}.Options()[0].ID)
}

func (service *NotificationSoundsService) Choose(event string, sound string) error {
	known := ChoiceUtils{}.All(service.Sounds[event] != "", NotificationSoundUtils{}.IsSound(sound))
	FlowUtils{}.When(known, func() { service.choose(event, sound) })()
	return nil
}

func (service *NotificationSoundsService) Preview(event string) {
	service.applicationEventPublisher.PublishEvent(SoundPreviewRequested{Event: event, Sound: service.Sound(event)}, false)
}

// @OnEvent
func (service *NotificationSoundsService) Previewed(finished SoundPreviewFinished) {
	silent := finished.Sound == NotificationSoundUtils{}.Options()[0].ID
	FlowUtils{}.When(!ChoiceUtils{}.Any(finished.Played, silent), func() { service.Error = "This browser could not play the preview." })()
}

func (service *NotificationSoundsService) choose(event string, sound string) {
	previous := service.Sounds
	next := maps.Clone(service.Sounds)
	next[event] = sound
	service.apply(next)
	FlowUtils{}.When(!service.Saving, func() { service.send(previous, next) })()
}

func (service *NotificationSoundsService) send(previous map[string]string, next map[string]string) {
	service.Saving = true
	service.Error = ""
	stored, failure := service.usersClient.SavePreferences(service.authorization, map[string]any{"notificationSounds": next})
	service.Saving = false
	FlowUtils{}.Either(failure == nil, func() { service.saved(stored) }, func() {
		service.Error = HTTPUtils{}.Message(failure, "Could not save notification sounds.", "Network error.")
		service.apply(previous)
	})()
}

func (service *NotificationSoundsService) saved(stored UserPreferences) {
	service.apply(NotificationSoundUtils{}.Normalize(stored.NotificationSounds))
	service.applicationEventPublisher.PublishEvent(ToastRequested{Message: "Notification sound saved."}, false)
}

func (service *NotificationSoundsService) apply(sounds map[string]string) {
	service.Sounds = sounds
}
