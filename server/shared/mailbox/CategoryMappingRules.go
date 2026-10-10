import "fmt"

// CategoryMappingRules maps the system roles (inbox, sent ...) of each
// assigned domain to physical folder paths. Stored mappings are read
// defensively: a missing role gets its native default and one path never
// carries two roles.
type CategoryMappingRules struct{}

// Normalized answers the mapping of every assigned domain.
func (rules CategoryMappingRules) Normalized(value any, assignedDomains []string) map[string]any {
	source, _ := RecordValues{}.Object(value)
	result := map[string]any{}
	for _, rawDomain := range assignedDomains {
		domain := Security{}.NormalizeDomain(rawDomain)
		if domain == "" {
			continue
		}
		supplied, _ := RecordValues{}.Object(source[domain])
		mapping := map[string]any{}
		used := map[string]bool{}
		for _, role := range rules.roles() {
			selected := CategoryPath{}.Normalized(supplied[role])
			if selected == "" || used[selected] {
				selected = rules.uniqueDefaultPath(role, used)
			}
			mapping[role] = selected
			used[selected] = true
		}
		result[domain] = mapping
	}
	return result
}

func (CategoryMappingRules) roles() []string {
	return []string{"inbox", "sent", "drafts", "spam", "archive", "trash"}
}

func (CategoryMappingRules) uniqueDefaultPath(role string, used map[string]bool) string {
	if !used[role] {
		return role
	}
	systemPath := "system/" + role
	if !used[systemPath] {
		return systemPath
	}
	suffix := 2
	for used[fmt.Sprintf("%s-%d", systemPath, suffix)] {
		suffix++
	}
	return fmt.Sprintf("%s-%d", systemPath, suffix)
}
