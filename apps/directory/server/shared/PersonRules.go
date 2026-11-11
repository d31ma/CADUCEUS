import "slices"

type PersonRules struct{}

func (rules PersonRules) With(person PersonDocument, domainId string) PersonDocument {
	person.DomainIds = rules.added(person.DomainIds, domainId)
	person.DefaultDomainId = rules.defaultOr(person.DefaultDomainId, domainId)
	return person
}

func (rules PersonRules) WithAdmin(person PersonDocument, domainId string, admin bool) PersonDocument {
	person.AdminDomainIds = rules.removed(person.AdminDomainIds, domainId)
	person.AdminDomainIds = rules.addedWhen(admin, person.AdminDomainIds, domainId)
	return person
}

func (rules PersonRules) Without(person PersonDocument, domainId string) PersonDocument {
	person.DomainIds = rules.removed(person.DomainIds, domainId)
	person.AdminDomainIds = rules.removed(person.AdminDomainIds, domainId)
	person.DefaultDomainId = rules.defaultAfter(person.DefaultDomainId, domainId, person.DomainIds)
	return person
}

func (rules PersonRules) Member(person PersonDocument, domainId string) bool {
	return slices.Contains(person.DomainIds, domainId)
}

func (rules PersonRules) added(domainIds []string, domainId string) []string {
	if slices.Contains(domainIds, domainId) {
		return slices.Clone(domainIds)
	}
	return append(slices.Clone(domainIds), domainId)
}

func (rules PersonRules) addedWhen(wanted bool, domainIds []string, domainId string) []string {
	if wanted {
		return rules.added(domainIds, domainId)
	}
	return domainIds
}

func (rules PersonRules) removed(domainIds []string, domainId string) []string {
	return slices.DeleteFunc(slices.Clone(domainIds), func(candidate string) bool { return candidate == domainId })
}

func (rules PersonRules) defaultOr(current string, domainId string) string {
	if current == "" {
		return domainId
	}
	return current
}

func (rules PersonRules) defaultAfter(current string, removedId string, remaining []string) string {
	if current != removedId {
		return current
	}
	return rules.first(remaining)
}

func (rules PersonRules) first(domainIds []string) string {
	if len(domainIds) == 0 {
		return ""
	}
	return domainIds[0]
}
