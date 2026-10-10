import "strings"

// NotificationSounds keeps the sound each mail event plays. Stored rows are
// read forgivingly: an unknown sound falls back to the event's default.
type NotificationSounds struct{}

// Normalized answers the sound of every event.
func (notification NotificationSounds) Normalized(value any) map[string]any {
	input := PreferenceChoices{}.Object(value)
	sounds := notification.sounds()
	defaults := map[string]string{"new-mail": "chime", "hit-send": "pop", "send": "pulse", "confirm": "bell"}
	normalized := map[string]any{}
	for event, fallback := range defaults {
		normalized[event] = PreferenceChoices{}.Text(input[event], sounds, fallback)
	}
	return normalized
}

// Checked answers the sounds a caller sent, or why they are refused.
func (sounds NotificationSounds) Checked(value any) (map[string]any, *PreferenceProblem) {
	input, isObject := RecordValues{}.Object(value)
	if !isObject {
		return nil, &PreferenceProblem{Message: "notificationSounds must be an object"}
	}
	for _, key := range (PreferenceChoices{}).Keys(input) {
		if !(PreferenceChoices{}).Includes(sounds.events(), key) {
			return nil, &PreferenceProblem{Message: "Unknown notification sound event: " + key}
		}
	}
	for _, event := range sounds.events() {
		raw, present := input[event]
		text, isText := raw.(string)
		if present && !(isText && (PreferenceChoices{}).Includes(sounds.sounds(), text)) {
			return nil, &PreferenceProblem{Message: event + " sound must be one of: " + strings.Join(sounds.sounds(), ", ")}
		}
	}
	return sounds.Normalized(input), nil
}

func (NotificationSounds) events() []string { return []string{"new-mail", "hit-send", "send", "confirm"} }

func (NotificationSounds) sounds() []string { return []string{"none", "chime", "bell", "pop", "pulse"} }
