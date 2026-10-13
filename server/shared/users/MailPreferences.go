import "fmt"

// MailPreferences keeps a user's reading and composing settings. Stored
// values are read without trusting their shape: each field falls back to its
// default, and sender-keyed settings keep only the user's own addresses.
type MailPreferences struct{}

// Normalized answers every mail preference for a user with the given
// primary address, aliases and domains.
func (preferences MailPreferences) Normalized(value any, email string, aliases []string, domains []string) map[string]any {
	input := PreferenceChoices{}.Object(value)
	senders := preferences.ownedSenders(email, aliases)
	choices := PreferenceChoices{}
	normalized := map[string]any{
		"workspaceLayout":    choices.Text(input["workspaceLayout"], []string{"split", "list", "split-vertical"}, "split"),
		"interfaceScale":     choices.Number(input["interfaceScale"], []float64{0.8, 0.9, 1, 1.1, 1.2}, 1),
		"markAsReadDelay":    choices.Number(input["markAsReadDelay"], []float64{0, 500, 2000, -1}, 500),
		"backspaceAction":    choices.Text(input["backspaceAction"], []string{"archive", "trash"}, "archive"),
		"conversationOrder":  choices.Text(input["conversationOrder"], []string{"ascending", "descending"}, "ascending"),
		"emailRenderMode":    choices.Text(input["emailRenderMode"], []string{"theme", "light", "dark"}, "theme"),
		"spellcheckLanguage": choices.Text(input["spellcheckLanguage"], preferences.spellcheckLanguages(), ""),
		"defaultSendAction":  choices.Text(input["defaultSendAction"], []string{"send", "send-and-archive"}, "send"),
		"defaultReplyAction": choices.Text(input["defaultReplyAction"], []string{"reply", "reply-all"}, "reply"),
		"undoSendDelay":      choices.Number(input["undoSendDelay"], []float64{0, 5000, 15000, 30000, 60000}, 5000),
		"defaultFrom":        preferences.ownedSender(input["defaultFrom"], senders),
		"autoCcBySender":     preferences.recipientMap(input["autoCcBySender"], senders),
		"autoBccBySender":    preferences.recipientMap(input["autoBccBySender"], senders),
		"findInMailDisabledDomains": preferences.disabledDomains(input["findInMailDisabledDomains"], domains),
	}
	for field, fallback := range preferences.flags() {
		normalized[field] = choices.Boolean(input[field], fallback)
	}
	return normalized
}

func (MailPreferences) flags() map[string]bool {
	return map[string]bool{
		"use24HourClock": false, "showUnreadForAllFolders": false, "detailedHeaders": false,
		"detailedNames": false, "restrictMessageWidth": false, "swipeEnabled": true,
		"richText": true, "spellcheck": true, "displayAttachmentPreviews": true, "unsnoozeToTop": true,
	}
}

func (MailPreferences) spellcheckLanguages() []string {
	return []string{"", "af", "bg", "ca", "cs", "da", "de", "el", "en-AU", "en-CA", "en-GB", "en-US",
		"es", "et", "fo", "fr", "he", "hi", "hr", "hu", "it", "ko", "lt", "lv", "nb", "nl",
		"pl", "pt", "pt-BR", "ro", "ru", "sk", "sl", "sq", "sr", "sv", "tr", "uk", "vi"}
}

func (MailPreferences) ownedSenders(email string, aliases []string) map[string]bool {
	senders := map[string]bool{}
	for _, address := range append([]string{email}, aliases...) {
		if normalized := (Security{}).NormalizeEmailAddress(address); normalized != "" {
			senders[normalized] = true
		}
	}
	return senders
}

func (MailPreferences) ownedSender(value any, senders map[string]bool) string {
	if value == nil || value == "" {
		return ""
	}
	sender := Security{}.NormalizeEmailAddress(fmt.Sprint(value))
	if !senders[sender] {
		return ""
	}
	return sender
}

// recipientMap keeps, for each owned sender, at most 50 distinct valid
// recipients, and at most 200 recipients overall.
func (preferences MailPreferences) recipientMap(value any, senders map[string]bool) map[string]any {
	input, isObject := RecordValues{}.Object(value)
	output := map[string]any{}
	if !isObject {
		return output
	}
	total := 0
	for _, rawSender := range (PreferenceChoices{}).Keys(input) {
		sender := Security{}.NormalizeEmailAddress(rawSender)
		list, isList := RecordValues{}.List(input[rawSender])
		if !senders[sender] || !isList {
			continue
		}
		recipients := preferences.recipients(list)
		total += len(recipients)
		if total > preferences.maximumRecipients() {
			break
		}
		if len(recipients) > 0 {
			output[sender] = recipients
		}
	}
	return output
}

func (preferences MailPreferences) recipients(list []any) []string {
	recipients := PreferenceChoices{}.Addresses(list)
	if len(recipients) > preferences.maximumRecipientsPerSender() {
		recipients = recipients[:preferences.maximumRecipientsPerSender()]
	}
	return recipients
}

func (MailPreferences) maximumRecipients() int { return 200 }

func (MailPreferences) maximumRecipientsPerSender() int { return 50 }

func (MailPreferences) disabledDomains(value any, domains []string) []string {
	list, isList := RecordValues{}.List(value)
	chosen := []string{}
	if !isList {
		return chosen
	}
	allowed := PreferenceChoices{}.Domains(domains)
	for _, item := range list {
		text, _ := item.(string)
		domain := Security{}.NormalizeDomain(text)
		if domain != "" && (PreferenceChoices{}).Includes(allowed, domain) && !(PreferenceChoices{}).Includes(chosen, domain) {
			chosen = append(chosen, domain)
		}
	}
	return chosen
}
