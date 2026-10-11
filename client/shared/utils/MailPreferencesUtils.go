import (
	"slices"
	"strings"
)

// MailPreferencesUtils reads the composer's part of the mail preferences on
// top of the shell's catalog check (AppearanceUtils.ChosenMail): the undo-send
// delay and the automatic CC and BCC recipients of each sender.
type MailPreferencesUtils struct{}

// Normalize keeps catalog values and well-formed recipient lists.
func (MailPreferencesUtils) Normalize(preferences *MailPreferences) MailPreferences {
	normalized := AppearanceUtils{}.ChosenMail(preferences)
	normalized.UndoSendDelay = MailPreferencesUtils{}.delay(normalized.UndoSendDelay)
	normalized.AutoCcBySender = MailPreferencesUtils{}.recipientMap(normalized.AutoCcBySender)
	normalized.AutoBccBySender = MailPreferencesUtils{}.recipientMap(normalized.AutoBccBySender)
	return normalized
}

// Defaults are the preferences before the server answered.
func (MailPreferencesUtils) Defaults() MailPreferences {
	return AppearanceUtils{}.DefaultMail()
}

func (MailPreferencesUtils) delay(delay int) int {
	if slices.Contains(MailPreferencesUtils{}.delays(), delay) {
		return delay
	}
	return MailPreferencesUtils{}.delays()[1]
}

func (MailPreferencesUtils) delays() []int {
	return []int{0, 5000, 15000, 30000, 60000}
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
