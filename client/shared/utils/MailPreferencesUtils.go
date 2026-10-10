import "slices"

// MailPreferencesUtils keeps the reader to the bounded catalog of mail
// preferences that mail-preferences.js defined.
type MailPreferencesUtils struct{}

func (MailPreferencesUtils) delays() []int { return []int{0, 500, 2000, -1} }

func (MailPreferencesUtils) orders() []string { return []string{"ascending", "descending"} }

func (MailPreferencesUtils) renderModes() []string { return []string{"theme", "light", "dark"} }

func (MailPreferencesUtils) replyActions() []string { return []string{"reply", "reply-all"} }

func (MailPreferencesUtils) Defaults() MailPreferences {
	return MailPreferences{MarkAsReadDelay: MailPreferencesUtils{}.delays()[1], ConversationOrder: MailPreferencesUtils{}.orders()[0], EmailRenderMode: MailPreferencesUtils{}.renderModes()[0], DefaultReplyAction: MailPreferencesUtils{}.replyActions()[0], DisplayAttachmentPreviews: true}
}

func (MailPreferencesUtils) Normalize(changed MailPreferencesChanged) MailPreferences {
	defaults := MailPreferencesUtils{}.Defaults()
	return MailPreferences{
		MarkAsReadDelay:           MailPreferencesUtils{}.delay(changed.MarkAsReadDelay, defaults.MarkAsReadDelay),
		DetailedHeaders:           changed.DetailedHeaders,
		DetailedNames:             changed.DetailedNames,
		ConversationOrder:         MailPreferencesUtils{}.choice(changed.ConversationOrder, MailPreferencesUtils{}.orders(), defaults.ConversationOrder),
		EmailRenderMode:           MailPreferencesUtils{}.choice(changed.EmailRenderMode, MailPreferencesUtils{}.renderModes(), defaults.EmailRenderMode),
		DefaultReplyAction:        MailPreferencesUtils{}.choice(changed.DefaultReplyAction, MailPreferencesUtils{}.replyActions(), defaults.DefaultReplyAction),
		DisplayAttachmentPreviews: changed.DisplayAttachmentPreviews,
	}
}

func (MailPreferencesUtils) delay(value int, fallback int) int {
	if slices.Contains(MailPreferencesUtils{}.delays(), value) {
		return value
	}
	return fallback
}

func (MailPreferencesUtils) choice(value string, allowed []string, fallback string) string {
	if slices.Contains(allowed, value) {
		return value
	}
	return fallback
}
