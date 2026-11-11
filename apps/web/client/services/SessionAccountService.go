import (
	"slices"
	"strings"
)

// @Service
type SessionAccountService struct {
	Accounts      []SessionAccount
	CurrentDomain string
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

func (service *SessionAccountService) Restore(signedIn []Me) {
	service.Accounts = service.accountsOf(signedIn)
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

func (service *SessionAccountService) accountsOf(signedIn []Me) []SessionAccount {
	accounts := []SessionAccount{}
	for _, me := range signedIn {
		accounts = append(accounts, service.accountOf(me))
	}
	return accounts
}

func (service *SessionAccountService) without(domainId string) []SessionAccount {
	return slices.DeleteFunc(slices.Clone(service.Accounts), func(account SessionAccount) bool {
		return account.DomainId == domainId
	})
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
