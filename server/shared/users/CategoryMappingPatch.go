// CategoryMappingPatch checks the category mappings a caller sends: every
// assigned domain must be present with every system role, each role on a
// safe folder path of its own.
type CategoryMappingPatch struct{}

// Checked answers the mappings, or why they are refused.
func (CategoryMappingPatch) Checked(value any, assignedDomains []string) (map[string]any, *PreferenceProblem) {
	input, isObject := RecordValues{}.Object(value)
	if !isObject {
		return nil, &PreferenceProblem{Message: "categoryMappings must be an object"}
	}
	allowed := []string{}
	for _, rawDomain := range assignedDomains {
		if domain := (Security{}).NormalizeDomain(rawDomain); domain != "" && !(PreferenceChoices{}).Includes(allowed, domain) {
			allowed = append(allowed, domain)
		}
	}
	for _, rawDomain := range (PreferenceChoices{}).Keys(input) {
		if domain := (Security{}).NormalizeDomain(rawDomain); domain == "" || !(PreferenceChoices{}).Includes(allowed, domain) {
			return nil, &PreferenceProblem{Message: "Category mapping domain is not assigned to this user", Forbidden: true}
		}
	}
	roles := CategoryMappingRules{}.roles()
	mappings := map[string]any{}
	for _, domain := range allowed {
		record, isRecord := RecordValues{}.Object(input[domain])
		if !isRecord {
			return nil, &PreferenceProblem{Message: "Category mappings for " + domain + " must include every system role"}
		}
		for role := range record {
			if !(PreferenceChoices{}).Includes(roles, role) {
				return nil, &PreferenceProblem{Message: "Category mappings contain an unknown system role"}
			}
		}
		mapping := map[string]any{}
		used := map[string]bool{}
		for _, role := range roles {
			path := CategoryPath{}.Normalized(record[role])
			if path == "" {
				return nil, &PreferenceProblem{Message: role + " must map to a valid folder path"}
			}
			if used[path] {
				return nil, &PreferenceProblem{Message: "Each system role must map to a different folder path"}
			}
			used[path] = true
			mapping[role] = path
		}
		mappings[domain] = mapping
	}
	return mappings, nil
}
