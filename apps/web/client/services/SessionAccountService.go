import "strings"

// @Service
type SessionAccountService struct {
	Accounts      []SessionAccount
	CurrentDomain string
	// @Needs
	identity *IdentityClient
}

func (service *SessionAccountService) Load(accessToken string) (Me, error) {
	return BehaviorsAnswer[Me](service.identity.Me("Bearer " + accessToken))
}

func (service *SessionAccountService) Add(me Me) {
	service.Accounts = append(service.without(me.DomainId), service.accountOf(me))
	service.Switch(me.DomainId)
}

func (service *SessionAccountService) Switch(domainId string) {
	service.CurrentDomain = domainId
	service.markCurrent()
}

func (service *SessionAccountService) Remove(domainId string) {
	service.Accounts = service.without(domainId)
	service.Switch(service.firstDomain())
}

func (service *SessionAccountService) Current() SessionAccount {
	return service.find(service.CurrentDomain)
}

func (service *SessionAccountService) accountOf(me Me) SessionAccount {
	return SessionAccount{
		DomainId:    me.DomainId,
		Address:     me.PrimaryAddress,
		DisplayName: me.DisplayName,
		Initials:    service.initials(me.DomainId),
	}
}

func (service *SessionAccountService) without(domainId string) []SessionAccount {
	kept := []SessionAccount{}
	for _, account := range service.Accounts {
		kept = service.keepOther(kept, account, domainId)
	}
	return kept
}

func (service *SessionAccountService) keepOther(kept []SessionAccount, account SessionAccount, domainId string) []SessionAccount {
	if account.DomainId == domainId {
		return kept
	}
	return append(kept, account)
}

func (service *SessionAccountService) markCurrent() {
	for index := range service.Accounts {
		service.Accounts[index].Current = service.Accounts[index].DomainId == service.CurrentDomain
	}
}

func (service *SessionAccountService) find(domainId string) SessionAccount {
	found := SessionAccount{}
	for _, account := range service.Accounts {
		found = service.matching(found, account, domainId)
	}
	return found
}

func (service *SessionAccountService) matching(found SessionAccount, account SessionAccount, domainId string) SessionAccount {
	if account.DomainId == domainId {
		return account
	}
	return found
}

func (service *SessionAccountService) firstDomain() string {
	if len(service.Accounts) == 0 {
		return ""
	}
	return service.Accounts[0].DomainId
}

func (service *SessionAccountService) initials(domainId string) string {
	letters := []rune(domainId + "  ")
	return strings.ToUpper(string(letters[0])) + string(letters[1])
}
