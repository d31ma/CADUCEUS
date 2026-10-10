import (
	"slices"
	"strings"
)

// MailPreferencesUtils normalizes the mail preferences the composer reads,
// like normalizeMailPreferences in client/shared/scripts/mail-preferences.js
// (only the composer fields are ported here).
type MailPreferencesUtils struct{}

// Normalize keeps catalog values and lowercases addresses.
func (MailPreferencesUtils) Normalize(preferences MailPreferences) MailPreferences {
	preferences.DefaultFrom = strings.ToLower(strings.TrimSpace(preferences.DefaultFrom))
	preferences.UndoSendDelay = MailPreferencesUtils{}.delay(preferences.UndoSendDelay)
	preferences.DefaultSendAction = MailPreferencesUtils{}.sendAction(preferences.DefaultSendAction)
	preferences.AutoCcBySender = MailPreferencesUtils{}.recipientMap(preferences.AutoCcBySender)
	preferences.AutoBccBySender = MailPreferencesUtils{}.recipientMap(preferences.AutoBccBySender)
	return preferences
}

// Defaults are the preferences before the server answered.
func (MailPreferencesUtils) Defaults() MailPreferences {
	return MailPreferences{RichText: true, Spellcheck: true, DefaultSendAction: MailPreferencesUtils{}.plainSend(), UndoSendDelay: MailPreferencesUtils{}.defaultDelay(), AutoCcBySender: map[string][]string{}, AutoBccBySender: map[string][]string{}}
}

func (MailPreferencesUtils) plainSend() string { return "send" }

func (MailPreferencesUtils) defaultDelay() float64 { return 5000 }

func (MailPreferencesUtils) delays() []float64 {
	return []float64{0, 5000, 15000, 30000, 60000}
}

func (MailPreferencesUtils) delay(delay float64) float64 {
	if slices.Contains(MailPreferencesUtils{}.delays(), delay) {
		return delay
	}
	return MailPreferencesUtils{}.defaultDelay()
}

func (MailPreferencesUtils) sendAction(action string) string {
	if action == "send-and-archive" {
		return action
	}
	return MailPreferencesUtils{}.plainSend()
}

func (MailPreferencesUtils) recipientMap(recipients map[string][]string) map[string][]string {
	normalized := map[string][]string{}
	for sender, addresses := range recipients {
		MailPreferencesUtils{}.addSender(normalized, strings.ToLower(strings.TrimSpace(sender)), addresses)
	}
	return normalized
}

func (MailPreferencesUtils) addSender(normalized map[string][]string, sender string, addresses []string) {
	if sender != "" {
		normalized[sender] = RecipientUtils{}.Split(RecipientUtils{}.MergeAutomatic(strings.Join(addresses, ","), nil, nil))
	}
}
