// NotificationSounds keeps the sound each mail event plays. Stored rows are
// read forgivingly: an unknown sound falls back to the event's default.
type NotificationSounds struct{}

// Normalized answers the sound of every event.
func (NotificationSounds) Normalized(value any) map[string]any {
	input := PreferenceChoices{}.Object(value)
	sounds := []string{"none", "chime", "bell", "pop", "pulse"}
	defaults := map[string]string{"new-mail": "chime", "hit-send": "pop", "send": "pulse", "confirm": "bell"}
	normalized := map[string]any{}
	for event, fallback := range defaults {
		normalized[event] = PreferenceChoices{}.Text(input[event], sounds, fallback)
	}
	return normalized
}
