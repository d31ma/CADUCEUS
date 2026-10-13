import (
	"regexp"
	"strings"
	"unicode/utf16"
)

// UsersRecords normalizes user documents. Every preference is checked against
// the user's current domains and addresses, so a damaged or stale field can
// never widen what the user reaches; unknown profile fields pass through.
type UsersRecords struct{}

// Normalized answers the user with every field in its normal form. A user
// without an id gets a new one.
func (records UsersRecords) Normalized(user map[string]any) map[string]any {
	values := RecordValues{}
	rawEmail := values.Text(user["email"])
	email := Security{}.NormalizeEmailAddress(rawEmail)
	if email == "" {
		email = strings.ToLower(Security{}.Trim(rawEmail))
	}
	aliasList, _ := values.List(values.Coalesce(user["aliases"], []any{}))
	aliases := records.Aliases(aliasList, email)
	domains := records.domains(user["domains"])
	normalized := StoredRecords{}.Without(user, "phones")
	fields := map[string]any{
		"id":                       values.Or(user["id"], Security{}.RandomUuid()),
		"email":                    email,
		"aliases":                  aliases,
		"domains":                  domains,
		"calendarVisibility":       PreferenceChoices{}.Text(user["calendarVisibility"], []string{"private", "busy", "full"}, "private"),
		"mailboxNames":             records.mailboxNames(values.Coalesce(user["mailboxNames"], map[string]any{}), domains),
		"categoryMappings":         CategoryMappingRules{}.Normalized(user["categoryMappings"], domains),
		"categoryCatalog":          CategoryCatalog{}.Normalized(user["categoryCatalog"], domains),
		"remoteImagePolicy":        PreferenceChoices{}.Text(user["remoteImagePolicy"], []string{"block", "allow"}, "block"),
		"trustedImageSenders":      records.trustedImageSenders(values.Coalesce(user["trustedImageSenders"], []any{})),
		"interfaceFont":            FontPreferences{}.InterfaceFont(user["interfaceFont"]),
		"composerFont":             FontPreferences{}.ComposerFont(user["composerFont"]),
		"interfaceLocale":          LocalePreferences{}.InterfaceLocale(user["interfaceLocale"]),
		"notificationSounds":       NotificationSounds{}.Normalized(user["notificationSounds"]),
		"themePreferences":         ThemePreferences{}.Normalized(user["themePreferences"]),
		"githubProfileEnrichment":  user["githubProfileEnrichment"] == true,
		"keyboardShortcutOverrides": KeyboardShortcuts{}.Overrides(user["keyboardShortcutOverrides"]),
		"mailPreferences":          MailPreferences{}.Normalized(user["mailPreferences"], email, aliases, domains),
		"mcp":                      McpPreferences{}.Normalized(user["mcp"], domains),
	}
	for _, group := range []map[string]any{
		fields,
		GrammarPreferences{}.Normalized(user),
		UnreadBadgePreferences{}.Normalized(user, domains),
		NewMailNotificationPreferences{}.Normalized(user, domains),
	} {
		for key, value := range group {
			normalized[key] = value
		}
	}
	return normalized
}

// Aliases answers the distinct valid aliases other than the primary address.
func (UsersRecords) Aliases(aliases []any, email string) []string {
	normalized := []string{}
	for _, alias := range aliases {
		text, _ := alias.(string)
		address := Security{}.NormalizeEmailAddress(text)
		if address != "" && (email == "" || address != email) && !(PreferenceChoices{}).Includes(normalized, address) {
			normalized = append(normalized, address)
		}
	}
	return normalized
}

// domains answers the distinct valid domains of the user, in order.
func (UsersRecords) domains(value any) []string {
	return PreferenceChoices{}.Domains(value)
}

// mailboxNames keeps the safe display labels of domains the user still has.
func (records UsersRecords) mailboxNames(value any, domains []string) map[string]any {
	names, isObject := RecordValues{}.Object(value)
	normalized := map[string]any{}
	if !isObject {
		return normalized
	}
	control := regexp.MustCompile(`[\x00-\x1f\x7f]`)
	for _, rawDomain := range (PreferenceChoices{}).Keys(names) {
		domain := Security{}.NormalizeDomain(rawDomain)
		name := Security{}.Trim(RecordValues{}.Text(names[rawDomain]))
		if domain == "" || !(PreferenceChoices{}).Includes(domains, domain) || name == "" {
			continue
		}
		if len(utf16.Encode([]rune(name))) > records.maximumMailboxNameLength() || control.MatchString(name) {
			continue
		}
		normalized[domain] = name
	}
	return normalized
}

func (UsersRecords) maximumMailboxNameLength() int { return 64 }

// trustedImageSenders keeps at most 500 distinct valid sender addresses.
func (records UsersRecords) trustedImageSenders(value any) []string {
	list, isList := RecordValues{}.List(value)
	senders := []string{}
	if !isList {
		return senders
	}
	senders = PreferenceChoices{}.Addresses(list)
	if len(senders) > records.maximumTrustedImageSenders() {
		senders = senders[:records.maximumTrustedImageSenders()]
	}
	return senders
}

func (UsersRecords) maximumTrustedImageSenders() int { return 500 }
