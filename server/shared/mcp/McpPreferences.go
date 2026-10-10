// McpPreferences keeps whether a user's mail is reachable over MCP, at which
// access level, for which domains, and which folders stay hidden. A user
// without stored preferences starts disabled and scoped to every assigned
// domain.
type McpPreferences struct{}

// Normalized answers the complete MCP preferences of a user.
func (preferences McpPreferences) Normalized(value any, assignedDomains []string) map[string]any {
	assigned := preferences.bounded(PreferenceChoices{}.Domains(assignedDomains))
	input := PreferenceChoices{}.Object(value)
	allowed := assigned
	if _, isList := (RecordValues{}).List(input["allowedDomains"]); isList {
		allowed = []string{}
		for _, domain := range (PreferenceChoices{}).Domains(input["allowedDomains"]) {
			if (PreferenceChoices{}).Includes(assigned, domain) {
				allowed = append(allowed, domain)
			}
		}
		allowed = preferences.bounded(allowed)
	}
	enabled, _ := input["enabled"].(bool)
	return map[string]any{
		"enabled":         enabled,
		"accessLevel":     PreferenceChoices{}.Text(input["accessLevel"], []string{"read-only", "read-write", "read-write-send"}, "read-only"),
		"allowedDomains":  allowed,
		"excludedFolders": preferences.excludedFolders(input["excludedFolders"], allowed),
	}
}

func (preferences McpPreferences) bounded(domains []string) []string {
	if len(domains) > preferences.maximumDomains() {
		return domains[:preferences.maximumDomains()]
	}
	return domains
}

// excludedFolders keeps safe folder paths of allowed domains: at most 100 per
// domain and 500 in all.
func (preferences McpPreferences) excludedFolders(value any, allowed []string) map[string]any {
	input, isObject := RecordValues{}.Object(value)
	result := map[string]any{}
	if !isObject {
		return result
	}
	total := 0
	for _, rawDomain := range (PreferenceChoices{}).Keys(input) {
		domain := Security{}.NormalizeDomain(rawDomain)
		folders, isList := RecordValues{}.List(input[rawDomain])
		if domain == "" || !(PreferenceChoices{}).Includes(allowed, domain) || !isList {
			continue
		}
		if len(folders) > preferences.maximumFoldersPerDomain() {
			folders = folders[:preferences.maximumFoldersPerDomain()]
		}
		kept := []string{}
		for _, rawFolder := range folders {
			folder := CategoryPath{}.Normalized(rawFolder)
			if folder != "" && !(PreferenceChoices{}).Includes(kept, folder) && total < preferences.maximumFolders() {
				kept = append(kept, folder)
				total++
			}
		}
		if len(kept) > 0 {
			result[domain] = kept
		}
	}
	return result
}

func (McpPreferences) maximumDomains() int { return 100 }

func (McpPreferences) maximumFoldersPerDomain() int { return 100 }

func (McpPreferences) maximumFolders() int { return 500 }
