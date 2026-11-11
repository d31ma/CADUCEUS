import "slices"

type AccountManager struct {
	Operator       bool
	AdminDomainIds []string
}

func (manager AccountManager) Manages(domainIds []string) bool {
	return manager.allowed(domainIds)
}

func (manager AccountManager) allowed(domainIds []string) bool {
	if manager.Operator {
		return true
	}
	return manager.adminOfAll(domainIds)
}

func (manager AccountManager) adminOfAll(domainIds []string) bool {
	return len(domainIds) > 0 && !slices.ContainsFunc(domainIds, manager.notAdmin)
}

func (manager AccountManager) notAdmin(domainId string) bool {
	return !slices.Contains(manager.AdminDomainIds, domainId)
}
