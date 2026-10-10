// CategoryRoles finds the physical folder a mailbox maps one system role
// (inbox, sent, drafts, spam, archive, trash) to, as the former
// CategoryMappingService.pathFor did. Mail reading owns the category
// service; this is the minimal private version delivery needs.
type CategoryRoles struct {
	Users *UsersHelper
}

// PathFor answers the owner's path for a role in one domain, the requested
// path itself when it is no system role or nothing maps it, or fallback when
// the requested path is not a valid path.
func (roles CategoryRoles) PathFor(ownerEmail string, domain string, requested string) string {
	normalized := CategoryPath{}.Normalized(requested)
	if normalized == "" {
		return requested
	}
	if !roles.role(normalized) {
		return normalized
	}
	_, user, err := roles.Users.FindByEmail(ownerEmail)
	if err != nil || user == nil {
		return normalized
	}
	mappings := CategoryMappingRules{}.Normalized(user["categoryMappings"], DeliveryFields{}.Strings(user["domains"]))
	mapping, _ := RecordValues{}.Object(mappings[domain])
	if path, _ := mapping[normalized].(string); path != "" {
		return path
	}
	return normalized
}

func (CategoryRoles) role(path string) bool {
	switch path {
	case "inbox", "sent", "drafts", "spam", "archive", "trash":
		return true
	}
	return false
}
