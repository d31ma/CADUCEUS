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
	service.label()
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
		MemberId:    me.MemberId,
		DomainId:    me.DomainId,
		Address:     me.PrimaryAddress,
		DisplayName: me.DisplayName,
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

func (service *SessionAccountService) label() {
	for index := range service.Accounts {
		domainId := service.Accounts[index].DomainId
		shared := service.countInitials(service.initials(domainId)) > 1
		service.Accounts[index].Initials = service.pick(shared, service.initialsWithEnding(domainId), service.initials(domainId))
	}
}

func (service *SessionAccountService) countInitials(initials string) int {
	count := 0
	for _, account := range service.Accounts {
		count += service.oneWhenSame(service.initials(account.DomainId), initials)
	}
	return count
}

func (service *SessionAccountService) oneWhenSame(left string, right string) int {
	if left == right {
		return 1
	}
	return 0
}

func (service *SessionAccountService) pick(first bool, chosen string, otherwise string) string {
	if first {
		return chosen
	}
	return otherwise
}

func (service *SessionAccountService) initialsWithEnding(domainId string) string {
	ending := domainId[strings.LastIndex(domainId, ".")+1:]
	return string([]rune(service.initials(domainId))[0]) + string([]rune(ending + " ")[0])
}

func (service *SessionAccountService) initials(domainId string) string {
	letters := []rune(domainId + "  ")
	return strings.ToUpper(string(letters[0])) + string(letters[1])
}
