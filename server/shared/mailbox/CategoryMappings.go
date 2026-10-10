import "strings"

// CategoryMappings holds one mailbox owner's system-role mappings per exact
// domain. Found is false when the owner has no user record; such an owner
// sees the native defaults and gets no folder filter.
type CategoryMappings struct {
	Found    bool
	Mappings map[string]any
}

// Of answers the mappings stored on a user record (nil when there is none).
func (CategoryMappings) Of(user map[string]any) CategoryMappings {
	if user == nil {
		return CategoryMappings{Mappings: map[string]any{}}
	}
	domains, _ := user["domains"].([]string)
	return CategoryMappings{Found: true, Mappings: CategoryMappingRules{}.Normalized(user["categoryMappings"], domains)}
}

// Roles answers the system roles in their fixed order.
func (CategoryMappings) Roles() []string {
	return CategoryMappingRules{}.roles()
}

// IsRole reports whether value names a system role.
func (mappings CategoryMappings) IsRole(value string) bool {
	for _, role := range mappings.Roles() {
		if role == value {
			return true
		}
	}
	return false
}

// DefaultMapping answers every role mapped to the folder of its own name.
func (mappings CategoryMappings) DefaultMapping() map[string]string {
	mapping := map[string]string{}
	for _, role := range mappings.Roles() {
		mapping[role] = role
	}
	return mapping
}

// ForDomain answers the role mapping of one domain, or the native defaults.
func (mappings CategoryMappings) ForDomain(domain string) map[string]string {
	stored, isObject := mappings.Mappings[domain].(map[string]any)
	if !isObject {
		return mappings.DefaultMapping()
	}
	mapping := map[string]string{}
	for role, path := range stored {
		mapping[role], _ = path.(string)
	}
	return mapping
}

// stored answers the path a domain's mapping gives a role, or "".
func (mappings CategoryMappings) stored(domain string, role string) string {
	stored, _ := mappings.Mappings[domain].(map[string]any)
	path, _ := stored[role].(string)
	return path
}

// RoleByPath answers the role each mapped path carries.
func (mappings CategoryMappings) RoleByPath(mapping map[string]string) map[string]string {
	roles := map[string]string{}
	for _, role := range mappings.Roles() {
		roles[mapping[role]] = role
	}
	return roles
}

// FolderFilter answers the physical folder each domain lists for a
// requested role, or nil when the owner is unknown or the folder is no role.
func (mappings CategoryMappings) FolderFilter(domains []string, requested string) map[string]string {
	if !mappings.Found || !mappings.IsRole(strings.ToLower(requested)) {
		return nil
	}
	filter := map[string]string{}
	for _, domain := range domains {
		filter[domain] = TextValue{}.Of(TextValue{}.Or(mappings.stored(domain, requested), requested))
	}
	return filter
}

// PathFor answers the physical folder of a requested folder: a role's
// mapped path, or the normalised folder itself ("" when it is unsafe).
func (mappings CategoryMappings) PathFor(domain string, requested any) string {
	normalized := CategoryPath{}.Normalized(requested)
	if normalized == "" || !mappings.IsRole(normalized) {
		return normalized
	}
	if path := mappings.stored(domain, normalized); path != "" {
		return path
	}
	return normalized
}

// PresentEmail answers the email with its normalised folder and the system
// role that folder carries (null when it carries none).
func (mappings CategoryMappings) PresentEmail(email map[string]any) map[string]any {
	text := TextValue{}
	path := CategoryPath{}.Normalized(text.Or(email["folder"], "inbox"))
	if path == "" {
		path = "inbox"
	}
	presented := text.Copy(email)
	presented["folder"] = path
	presented["systemFolder"] = nil
	if role := mappings.RoleByPath(mappings.ForDomain(text.Of(email["domain"])))[path]; role != "" {
		presented["systemFolder"] = role
	}
	return presented
}

// PresentEmails answers every email presented with PresentEmail.
func (mappings CategoryMappings) PresentEmails(emails []map[string]any) []map[string]any {
	presented := []map[string]any{}
	for _, email := range emails {
		presented = append(presented, mappings.PresentEmail(email))
	}
	return presented
}
