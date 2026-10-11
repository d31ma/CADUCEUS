import "cmp"

// NotificationSoundUtils is the Go form of the catalog and normalization in
// shared/scripts/notification-sounds.js. Playing a cue stays in the page.
type NotificationSoundUtils struct{}

func (utils NotificationSoundUtils) Events() []SoundEvent {
	sounds := utils.Options()
	return []SoundEvent{
		{ID: "new-mail", Label: "New mail", Description: "A message arrives while CADUCEUS is visible.", Default: sounds[1].ID},
		{ID: "hit-send", Label: "Send pressed", Description: "The send action is accepted for processing.", Default: sounds[3].ID},
		{ID: "send", Label: "Message sent", Description: "The server accepts the outgoing message.", Default: sounds[4].ID},
		{ID: "confirm", Label: "Action confirmed", Description: "An undo or similar action completes.", Default: sounds[2].ID},
	}
}

func (NotificationSoundUtils) Options() []SoundOption {
	return []SoundOption{
		{ID: "none", Label: "None"}, {ID: "chime", Label: "Chime"}, {ID: "bell", Label: "Bell"},
		{ID: "pop", Label: "Pop"}, {ID: "pulse", Label: "Pulse"},
	}
}

// Normalize keeps a known sound for every event and the default otherwise.
func (utils NotificationSoundUtils) Normalize(chosen map[string]string) map[string]string {
	return utils.sounds(chosen, utils.soundIds())
}

func (utils NotificationSoundUtils) IsSound(id string) bool {
	return utils.soundIds()[id] != ""
}

func (utils NotificationSoundUtils) sounds(chosen map[string]string, known map[string]string) map[string]string {
	sounds := map[string]string{}
	for _, event := range utils.Events() {
		sounds[event.ID] = cmp.Or(known[chosen[event.ID]], event.Default)
	}
	return sounds
}

func (utils NotificationSoundUtils) soundIds() map[string]string {
	known := map[string]string{}
	for _, option := range utils.Options() {
		known[option.ID] = option.ID
	}
	return known
}
