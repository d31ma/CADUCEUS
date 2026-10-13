// UsersMerge combines a stored user with a newly normalized one, keeping the
// choices the former repository kept: the stored id, settings the stored user
// already made, and fields the caller did not name.
type UsersMerge struct {
	Existing   map[string]any
	Normalized map[string]any
	// Given is the user as the caller passed it, before normalization: a
	// field it leaves out keeps the stored value.
	Given map[string]any
}

// Merged answers {...existing, ...normalized} with the kept choices.
func (merge UsersMerge) Merged() map[string]any {
	values := RecordValues{}
	existing, normalized := merge.Existing, merge.Normalized
	merged := StoredRecords{}.Without(existing)
	for key, value := range normalized {
		merged[key] = value
	}
	merged["id"] = values.Or(existing["id"], normalized["id"])
	merged["aliases"] = merge.nonEmpty(normalized["aliases"], values.Or(existing["aliases"], []any{}))
	merged["mailboxNames"] = merge.nonEmptyObject(normalized["mailboxNames"], values.Coalesce(existing["mailboxNames"], map[string]any{}))
	merged["trustedImageSenders"] = merge.nonEmpty(normalized["trustedImageSenders"], values.Coalesce(existing["trustedImageSenders"], []any{}))
	merged["grammarCheckDisabledRules"] = merge.nonEmpty(existing["grammarCheckDisabledRules"], normalized["grammarCheckDisabledRules"])
	for _, field := range []string{"calendarVisibility", "categoryMappings", "remoteImagePolicy", "interfaceFont", "composerFont",
		"notificationSounds", "themePreferences", "grammarCheckLanguage", "mcp"} {
		merged[field] = values.Or(existing[field], normalized[field])
	}
	for _, field := range []string{"githubProfileEnrichment", "grammarCheckEnabled"} {
		merged[field] = values.Coalesce(existing[field], normalized[field])
	}
	for _, field := range []string{"categoryCatalog", "interfaceLocale", "unreadBadgeScope", "unreadBadgeDomain",
		"newMailNotificationDomains", "newMailNotificationPrivacy", "newMailNotificationActions",
		"keyboardShortcutOverrides", "mailPreferences"} {
		merged[field] = merge.named(field)
	}
	return merged
}

// Preferences answers the display-only preference fields of the normalized
// user, as one patch.
func (merge UsersMerge) Preferences() map[string]any {
	values := RecordValues{}
	normalized := merge.Normalized
	patch := map[string]any{
		"mailboxNames":        values.Coalesce(normalized["mailboxNames"], map[string]any{}),
		"categoryMappings":    values.Coalesce(normalized["categoryMappings"], map[string]any{}),
		"categoryCatalog":     values.Coalesce(normalized["categoryCatalog"], map[string]any{}),
		"trustedImageSenders": values.Coalesce(normalized["trustedImageSenders"], []any{}),
	}
	for _, field := range []string{"calendarVisibility", "remoteImagePolicy", "interfaceFont", "composerFont", "interfaceLocale",
		"notificationSounds", "themePreferences", "githubProfileEnrichment", "grammarCheckEnabled", "grammarCheckLanguage",
		"grammarCheckDisabledRules", "unreadBadgeScope", "unreadBadgeDomain", "newMailNotificationDomains",
		"newMailNotificationPrivacy", "newMailNotificationActions", "keyboardShortcutOverrides", "mailPreferences", "mcp"} {
		patch[field] = normalized[field]
	}
	return patch
}

// named answers the normalized field when the caller named it, else the
// stored one.
func (merge UsersMerge) named(field string) any {
	if _, present := merge.Given[field]; present {
		return merge.Normalized[field]
	}
	return merge.Existing[field]
}

// nonEmpty answers value when it is a list with items, else fallback.
func (UsersMerge) nonEmpty(value any, fallback any) any {
	if list, _ := (RecordValues{}).List(value); len(list) > 0 {
		return value
	}
	return fallback
}

// nonEmptyObject answers value when it is an object with keys, else fallback.
func (UsersMerge) nonEmptyObject(value any, fallback any) any {
	if object, _ := (RecordValues{}).Object(value); len(object) > 0 {
		return value
	}
	return fallback
}
