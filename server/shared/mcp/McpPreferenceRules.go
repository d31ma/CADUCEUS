import "fmt"

// McpPreferenceRules strictly validates a complete MCP configuration: unknown
// fields and domains the user is not assigned fail closed, and exclusions
// may only name safe folder paths of an allowed domain.
type McpPreferenceRules struct{}

// Validated answers the configuration in its stored form, or why it is
// refused.
func (rules McpPreferenceRules) Validated(input any, assignedDomains []string) (map[string]any, *McpPreferenceError) {
	value, isObject := RecordValues{}.Object(input)
	if !isObject {
		return nil, rules.refused("MCP configuration must be an object")
	}
	for field := range value {
		if !(PreferenceChoices{}).Includes([]string{"enabled", "accessLevel", "allowedDomains", "excludedFolders"}, field) {
			return nil, rules.refused("MCP configuration contains an unknown field")
		}
	}
	enabled, isFlag := value["enabled"].(bool)
	if !isFlag {
		return nil, rules.refused("enabled must be a boolean")
	}
	accessLevel, _ := value["accessLevel"].(string)
	if !(PreferenceChoices{}).Includes(rules.accessLevels(), accessLevel) {
		return nil, rules.refused("accessLevel must be one of: read-only, read-write, read-write-send")
	}
	allowedDomains, failure := rules.allowedDomains(value["allowedDomains"], assignedDomains)
	if failure != nil {
		return nil, failure
	}
	if enabled && len(allowedDomains) == 0 {
		return nil, rules.refused("At least one allowed domain is required while MCP is enabled")
	}
	excludedFolders, failure := rules.excludedFolders(value["excludedFolders"], allowedDomains)
	if failure != nil {
		return nil, failure
	}
	return map[string]any{"enabled": enabled, "accessLevel": accessLevel, "allowedDomains": allowedDomains, "excludedFolders": excludedFolders}, nil
}

func (rules McpPreferenceRules) allowedDomains(value any, assignedDomains []string) ([]string, *McpPreferenceError) {
	list, isList := RecordValues{}.List(value)
	if !isList || len(list) > rules.maximumDomains() {
		return nil, rules.refused(fmt.Sprintf("allowedDomains must contain at most %d domains", rules.maximumDomains()))
	}
	assigned := PreferenceChoices{}.Domains(assignedDomains)
	allowed := []string{}
	for _, raw := range list {
		text, isText := raw.(string)
		if !isText {
			return nil, rules.refused("Each allowed domain must be text")
		}
		domain := Security{}.NormalizeDomain(text)
		if domain == "" || !(PreferenceChoices{}).Includes(assigned, domain) {
			return nil, &McpPreferenceError{Message: "MCP domain is not assigned to this user", Forbidden: true}
		}
		if !(PreferenceChoices{}).Includes(allowed, domain) {
			allowed = append(allowed, domain)
		}
	}
	return allowed, nil
}

func (rules McpPreferenceRules) excludedFolders(value any, allowedDomains []string) (map[string]any, *McpPreferenceError) {
	source, isObject := RecordValues{}.Object(value)
	if !isObject {
		return nil, rules.refused("excludedFolders must be an object")
	}
	excluded := map[string]any{}
	total := 0
	for _, rawDomain := range (PreferenceChoices{}).Keys(source) {
		domain := Security{}.NormalizeDomain(rawDomain)
		if domain == "" || !(PreferenceChoices{}).Includes(allowedDomains, domain) {
			return nil, &McpPreferenceError{Message: "Excluded folders must belong to an allowed domain", Forbidden: true}
		}
		rawFolders, isList := RecordValues{}.List(source[rawDomain])
		if !isList || len(rawFolders) > rules.maximumFoldersPerDomain() {
			return nil, rules.refused(fmt.Sprintf("Each domain may exclude at most %d folders", rules.maximumFoldersPerDomain()))
		}
		folders := []string{}
		for _, rawFolder := range rawFolders {
			folder := CategoryPath{}.Normalized(rawFolder)
			if folder == "" {
				return nil, rules.refused("Excluded folders must use safe bounded folder paths")
			}
			if !(PreferenceChoices{}).Includes(folders, folder) {
				folders = append(folders, folder)
			}
		}
		total += len(folders)
		if total > rules.maximumFolders() {
			return nil, rules.refused(fmt.Sprintf("At most %d folders may be excluded", rules.maximumFolders()))
		}
		if len(folders) > 0 {
			excluded[domain] = folders
		}
	}
	return excluded, nil
}

func (McpPreferenceRules) refused(message string) *McpPreferenceError {
	return &McpPreferenceError{Message: message}
}

func (McpPreferenceRules) accessLevels() []string {
	return []string{"read-only", "read-write", "read-write-send"}
}

func (McpPreferenceRules) maximumDomains() int { return 100 }

func (McpPreferenceRules) maximumFoldersPerDomain() int { return 100 }

func (McpPreferenceRules) maximumFolders() int { return 500 }
