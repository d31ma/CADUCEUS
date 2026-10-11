import "strings"

// EmailAliasesService manages the account's extra addresses. Its primary
// address and aliases are also the sender identities other sections offer.
// @Service
type EmailAliasesService struct {
	// @Needs
	usersClient               *UsersClient
	applicationEventPublisher ApplicationEventPublisher
	authorization             string
	PrimaryEmail              string
	Aliases                   []string
	// @SessionState
	NewAlias    string
	AliasError  string
	AliasSaving bool
}

// @OnStart
func (service *EmailAliasesService) Start() {
	service.Aliases = []string{}
}

// @OnEvent({"replay":true})
func (service *EmailAliasesService) HearSession(changed SessionChanged) {
	service.authorization = "Bearer " + changed.Token
	service.PrimaryEmail = changed.Email
	FlowUtils{}.When(changed.SignedIn, service.load)()
}

func (service *EmailAliasesService) Add() error {
	alias := strings.TrimSpace(service.NewAlias)
	FlowUtils{}.When(alias != "", func() { service.add(alias) })()
	return nil
}

func (service *EmailAliasesService) Remove(alias string) error {
	list, failure := service.usersClient.RemoveAlias(service.authorization, service.aliasBody(alias))
	FlowUtils{}.Either(failure == nil, func() { service.showRemoved(list) }, func() {
		service.AliasError = HTTPUtils{}.Message(failure, "Could not remove alias.", service.network())
	})()
	return nil
}

// SenderAddresses are the primary address and every alias, once each.
func (service *EmailAliasesService) SenderAddresses() []string {
	return PreferenceUtils{}.Unique(append([]string{service.PrimaryEmail}, service.Aliases...))
}

func (service *EmailAliasesService) load() {
	list, failure := service.usersClient.Aliases(service.authorization)
	FlowUtils{}.When(failure == nil, func() { service.show(list) })()
	service.share()
	service.applicationEventPublisher.PublishEvent(SettingsActivity{Section: "aliases", Refused: HTTPUtils{}.Refused(failure)}, false)
}

func (service *EmailAliasesService) add(alias string) {
	service.AliasSaving = true
	service.AliasError = ""
	list, failure := service.usersClient.AddAlias(service.authorization, service.aliasBody(alias))
	service.AliasSaving = false
	FlowUtils{}.Either(failure == nil, func() { service.showAdded(list) }, func() {
		service.AliasError = HTTPUtils{}.Message(failure, "Could not add alias.", service.network())
	})()
}

func (service *EmailAliasesService) show(list AliasList) {
	service.PrimaryEmail = TextUtils{}.FirstOf(list.Email, service.PrimaryEmail)
	service.Aliases = append([]string{}, list.Aliases...)
}

func (service *EmailAliasesService) showAdded(list AliasList) {
	service.show(list)
	service.NewAlias = ""
	service.share()
	service.applicationEventPublisher.PublishEvent(ToastRequested{Message: "Alias added."}, false)
}

func (service *EmailAliasesService) showRemoved(list AliasList) {
	service.show(list)
	service.share()
	service.applicationEventPublisher.PublishEvent(ToastRequested{Message: "Alias removed."}, false)
}

// share tells the other sections which sender identities exist.
func (service *EmailAliasesService) share() {
	service.applicationEventPublisher.PublishEvent(SenderAddresses{Addresses: service.SenderAddresses()}, true)
}

func (service *EmailAliasesService) aliasBody(alias string) BehaviorValue {
	return map[string]any{"alias": alias}
}

func (service *EmailAliasesService) network() string { return "Network error." }
