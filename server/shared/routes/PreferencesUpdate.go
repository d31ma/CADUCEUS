// PreferencesUpdate checks the display preferences a body names, in the
// order the former route checked them, and writes each accepted value to
// Next. The first refusal stops it.
type PreferencesUpdate struct {
	Body map[string]any
	// User is the stored user; Next is the copy that receives the update.
	User map[string]any
	Next map[string]any
}

// Applied answers why the update is refused, or nil once every named
// preference is in Next.
func (update PreferencesUpdate) Applied() *PreferenceProblem {
	for _, step := range []func() *PreferenceProblem{update.choices, update.lists, update.catalogs, update.grammar} {
		if problem := step(); problem != nil {
			return problem
		}
	}
	badge, problem := UnreadBadgePreferences{}.Checked(update.Body, update.User, update.User["domains"])
	if problem != nil {
		return problem
	}
	for key, value := range badge {
		update.Next[key] = value
	}
	return nil
}

func (update PreferencesUpdate) choices() *PreferenceProblem {
	if visibility, isText := update.Body["calendarVisibility"].(string); isText {
		if !(PreferenceChoices{}).Includes([]string{"private", "busy", "full"}, visibility) {
			return &PreferenceProblem{Message: "calendarVisibility must be one of: private, busy, full"}
		}
		update.Next["calendarVisibility"] = visibility
	}
	domains := TextList{}.Of(update.User["domains"])
	if raw, named := update.Body["mailboxNames"]; named {
		names, problem := PreferenceChecks{}.MailboxNames(raw, domains)
		if problem != nil {
			return problem
		}
		update.Next["mailboxNames"] = names
	}
	if raw, named := update.Body["categoryMappings"]; named {
		mappings, problem := CategoryMappingPatch{}.Checked(raw, domains)
		if problem != nil {
			return problem
		}
		update.Next["categoryMappings"] = mappings
	}
	if raw, named := update.Body["remoteImagePolicy"]; named {
		policy, isText := raw.(string)
		if !isText || !(PreferenceChoices{}).Includes([]string{"block", "allow"}, policy) {
			return &PreferenceProblem{Message: "remoteImagePolicy must be one of: block, allow"}
		}
		update.Next["remoteImagePolicy"] = policy
	}
	return nil
}

func (update PreferencesUpdate) lists() *PreferenceProblem {
	if raw, named := update.Body["trustedImageSenders"]; named {
		senders, problem := PreferenceChecks{}.TrustedImageSenders(raw)
		if problem != nil {
			return problem
		}
		update.Next["trustedImageSenders"] = senders
	}
	fonts := FontPreferences{}
	if raw, named := update.Body["interfaceFont"]; named {
		if !fonts.IsInterfaceFont(raw) {
			return &PreferenceProblem{Message: "interfaceFont must be one of: plex-sans, system, humanist, serif, monospace"}
		}
		update.Next["interfaceFont"] = raw
	}
	if raw, named := update.Body["composerFont"]; named {
		if !fonts.IsComposerFont(raw) {
			return &PreferenceProblem{Message: "composerFont must be one of: interface, plex-sans, system, humanist, serif, monospace"}
		}
		update.Next["composerFont"] = raw
	}
	if raw, named := update.Body["interfaceLocale"]; named {
		locale, problem := LocalePreferences{}.Checked(raw)
		if problem != nil {
			return problem
		}
		update.Next["interfaceLocale"] = locale
	}
	return nil
}

func (update PreferencesUpdate) catalogs() *PreferenceProblem {
	if raw, named := update.Body["notificationSounds"]; named {
		sounds, problem := NotificationSounds{}.Checked(raw)
		if problem != nil {
			return problem
		}
		update.Next["notificationSounds"] = sounds
	}
	if raw, named := update.Body["themePreferences"]; named {
		themes, problem := ThemePreferences{}.Checked(raw)
		if problem != nil {
			return problem
		}
		update.Next["themePreferences"] = themes
	}
	if raw, named := update.Body["githubProfileEnrichment"]; named {
		if _, isFlag := raw.(bool); !isFlag {
			return &PreferenceProblem{Message: "githubProfileEnrichment must be a boolean"}
		}
		update.Next["githubProfileEnrichment"] = raw
	}
	if raw, named := update.Body["keyboardShortcutOverrides"]; named {
		overrides, problem := KeyboardShortcuts{}.Checked(raw)
		if problem != nil {
			return problem
		}
		update.Next["keyboardShortcutOverrides"] = overrides
	}
	return nil
}

func (update PreferencesUpdate) grammar() *PreferenceProblem {
	if problem := (GrammarPreferences{}).Checked(update.Body); problem != nil {
		return problem
	}
	for _, field := range []string{"grammarCheckEnabled", "grammarCheckLanguage"} {
		if raw, named := update.Body[field]; named {
			update.Next[field] = raw
		}
	}
	if raw, named := update.Body["grammarCheckDisabledRules"]; named {
		update.Next["grammarCheckDisabledRules"] = TextList{}.Distinct(TextList{}.Of(raw))
	}
	return nil
}
