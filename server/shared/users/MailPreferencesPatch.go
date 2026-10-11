// MailPreferencesPatch checks a caller's complete or partial mail preference
// update for one user and merges it over the user's current preferences.
type MailPreferencesPatch struct {
	Email   string
	Aliases []string
	Domains []string
	// Current is the user's stored mailPreferences.
	Current any
}

// Checked answers the merged preferences, or why the patch is refused.
func (patch MailPreferencesPatch) Checked(value any) (map[string]any, *PreferenceProblem) {
	input, isObject := RecordValues{}.Object(value)
	if !isObject {
		return nil, &PreferenceProblem{Message: "mailPreferences must be an object"}
	}
	defaults := patch.normalized(nil)
	keys := PreferenceChoices{}.Keys(input)
	for _, key := range keys {
		if _, known := defaults[key]; !known {
			return nil, &PreferenceProblem{Message: "Unknown mail preference: " + key}
		}
	}
	for _, key := range keys {
		if !patch.valid(key, input[key]) {
			return nil, &PreferenceProblem{Message: "Invalid mail preference: " + key}
		}
	}
	candidate := patch.normalized(patch.Current)
	for key, field := range input {
		candidate[key] = field
	}
	return patch.normalized(candidate), nil
}

func (patch MailPreferencesPatch) normalized(value any) map[string]any {
	return MailPreferences{}.Normalized(value, patch.Email, patch.Aliases, patch.Domains)
}

// valid reports whether one field holds a value the catalog allows. A
// catalog field is valid when normalizing it alone keeps it unchanged, since
// every fallback is itself a catalog value.
func (patch MailPreferencesPatch) valid(key string, value any) bool {
	preferences := MailPreferences{}
	if _, isFlag := preferences.flags()[key]; isFlag {
		_, isBoolean := value.(bool)
		return isBoolean
	}
	senders := preferences.ownedSenders(patch.Email, patch.Aliases)
	switch key {
	case "defaultFrom":
		text, isText := value.(string)
		return isText && (text == "" || senders[Security{}.NormalizeEmailAddress(text)])
	case "autoCcBySender", "autoBccBySender":
		return patch.validRecipients(value, senders)
	case "findInMailDisabledDomains":
		return patch.validDomains(value)
	}
	_, isText := value.(string)
	_, isNumber := RecordValues{}.Number(value)
	return (isText || isNumber) && patch.normalized(map[string]any{key: value})[key] == value
}

func (patch MailPreferencesPatch) validRecipients(value any, senders map[string]bool) bool {
	input, isObject := RecordValues{}.Object(value)
	if !isObject {
		return false
	}
	preferences := MailPreferences{}
	total := 0
	for rawSender, rawRecipients := range input {
		recipients, isList := RecordValues{}.List(rawRecipients)
		if !senders[(Security{}).NormalizeEmailAddress(rawSender)] || !isList || len(recipients) > preferences.maximumRecipientsPerSender() {
			return false
		}
		for _, recipient := range recipients {
			text, isText := recipient.(string)
			if !isText || (Security{}).NormalizeEmailAddress(text) == "" {
				return false
			}
			total++
			if total > preferences.maximumRecipients() {
				return false
			}
		}
	}
	return true
}

func (patch MailPreferencesPatch) validDomains(value any) bool {
	list, isList := RecordValues{}.List(value)
	if !isList || len(list) > patch.maximumDisabledDomains() {
		return false
	}
	allowed := PreferenceChoices{}.Domains(patch.Domains)
	for _, item := range list {
		text, isText := item.(string)
		domain := Security{}.NormalizeDomain(text)
		if !isText || domain == "" || !(PreferenceChoices{}).Includes(allowed, domain) {
			return false
		}
	}
	return true
}

func (MailPreferencesPatch) maximumDisabledDomains() int { return 100 }
