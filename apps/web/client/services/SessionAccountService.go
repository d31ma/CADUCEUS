import (
	"slices"
	"strings"
)

// @Service
type SessionAccountService struct {
	Accounts      []SessionAccount
	CurrentDomain string
	Username      string
	DisplayName   string
	seen          []string
}

func (service *SessionAccountService) Apply(session Session) {
	service.seeAllWhenFirst(session)
	service.Username = session.Me.Username
	service.DisplayName = session.Me.DisplayName
	service.CurrentDomain = session.Me.DomainId
	service.Accounts = service.defaultFirst(service.accountsOf(session))
	service.label()
}

func (service *SessionAccountService) Remove(domainId string) {
	service.Accounts = service.without(domainId)
}

func (service *SessionAccountService) MarkDefault(domainId string) {
	service.markDefault(domainId)
	service.Accounts = service.defaultFirst(service.Accounts)
}

func (service *SessionAccountService) markDefault(domainId string) {
	for index := range service.Accounts {
		service.Accounts[index].Default = service.Accounts[index].DomainId == domainId
	}
}

func (service *SessionAccountService) MarkSeen() {
	service.seen = service.domainIds()
	service.clearNew()
}

func (service *SessionAccountService) Clear() {
	service.seen = []string{}
	service.Accounts = []SessionAccount{}
	service.CurrentDomain = ""
	service.Username = ""
	service.DisplayName = ""
}

func (service *SessionAccountService) Current() SessionAccount {
	return service.find(service.CurrentDomain)
}

func (service *SessionAccountService) Default() SessionAccount {
	return service.find(service.defaultDomain())
}

func (service *SessionAccountService) DomainList() string {
	return strings.Join(service.domainIds(), ",")
}

func (service *SessionAccountService) Single() bool {
	return len(service.Accounts) == 1
}

func (service *SessionAccountService) OtherDomain() string {
	others := service.without(service.defaultDomain())
	return service.firstDomainOf(others)
}

func (service *SessionAccountService) seeAllWhenFirst(session Session) {
	if len(service.seen) > 0 {
		return
	}
	service.seen = service.choiceIds(session.Domains)
}

func (service *SessionAccountService) choiceIds(choices []DomainChoice) []string {
	domainIds := []string{}
	for _, choice := range choices {
		domainIds = append(domainIds, choice.DomainId)
	}
	return domainIds
}

func (service *SessionAccountService) clearNew() {
	for index := range service.Accounts {
		service.Accounts[index].New = false
	}
}

func (service *SessionAccountService) accountsOf(session Session) []SessionAccount {
	accounts := []SessionAccount{}
	for _, choice := range session.Domains {
		accounts = append(accounts, service.accountOf(session, choice))
	}
	return accounts
}

func (service *SessionAccountService) accountOf(session Session, choice DomainChoice) SessionAccount {
	return SessionAccount{
		MemberId:    session.Me.MemberId,
		DomainId:    choice.DomainId,
		Address:     session.Me.Username + "@" + choice.DomainId,
		DisplayName: session.Me.DisplayName,
		Current:     choice.DomainId == session.Me.DomainId,
		Admin:       choice.Admin,
		Default:     choice.Default,
		New:         !slices.Contains(service.seen, choice.DomainId),
	}
}

func (service *SessionAccountService) defaultFirst(accounts []SessionAccount) []SessionAccount {
	sorted := slices.Clone(accounts)
	slices.SortStableFunc(sorted, func(left SessionAccount, right SessionAccount) int {
		return service.rank(left) - service.rank(right)
	})
	return sorted
}

func (service *SessionAccountService) rank(account SessionAccount) int {
	if account.Default {
		return 0
	}
	return 1
}

func (service *SessionAccountService) domainIds() []string {
	domainIds := []string{}
	for _, account := range service.Accounts {
		domainIds = append(domainIds, account.DomainId)
	}
	return domainIds
}

func (service *SessionAccountService) defaultDomain() string {
	domainId := ""
	for _, account := range service.Accounts {
		domainId = service.defaultOr(account, domainId)
	}
	return domainId
}

func (service *SessionAccountService) defaultOr(account SessionAccount, domainId string) string {
	if account.Default {
		return account.DomainId
	}
	return domainId
}

func (service *SessionAccountService) firstDomainOf(accounts []SessionAccount) string {
	if len(accounts) == 0 {
		return ""
	}
	return accounts[0].DomainId
}

func (service *SessionAccountService) without(domainId string) []SessionAccount {
	return slices.DeleteFunc(slices.Clone(service.Accounts), func(account SessionAccount) bool {
		return account.DomainId == domainId
	})
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
