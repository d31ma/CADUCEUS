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

// @OnEvent({"replay":true})
func (service *EmailAliasesService) Open(session AccountSession) error {
	service.authorization = "Bearer " + session.Token
	list, failure := service.usersClient.Aliases(service.authorization)
	service.showLoaded(list, failure)
	service.share()
	service.applicationEventPublisher.PublishEvent(SettingsActivity{Section: "aliases"}, false)
	return nil
}

func (service *EmailAliasesService) Add() error {
	alias := strings.TrimSpace(service.NewAlias)
	FlowUtils{}.When(alias != "", func() { service.add(alias) })
	return nil
}

func (service *EmailAliasesService) Remove(alias string) error {
	list, failure := service.usersClient.RemoveAlias(service.authorization, service.aliasBody(alias))
	service.showRemoved(list, failure)
	return nil
}

// SenderAddresses are the primary address and every alias, once each.
func (service *EmailAliasesService) SenderAddresses() []string {
	return PreferenceUtils{}.Unique(append([]string{service.PrimaryEmail}, service.Aliases...))
}

func (service *EmailAliasesService) add(alias string) {
	service.AliasSaving = true
	service.AliasError = ""
	list, failure := service.usersClient.AddAlias(service.authorization, service.aliasBody(alias))
	service.AliasSaving = false
	service.showAdded(list, failure)
}

func (service *EmailAliasesService) showLoaded(list AliasList, failure error) {
	if failure != nil {
		service.Aliases = []string{}
		return
	}
	service.PrimaryEmail = list.Email
	service.Aliases = list.Aliases
}

func (service *EmailAliasesService) showAdded(list AliasList, failure error) {
	if failure != nil {
		service.AliasError = "Could not add alias."
		return
	}
	service.PrimaryEmail = list.Email
	service.Aliases = list.Aliases
	service.NewAlias = ""
	service.share()
	service.applicationEventPublisher.PublishEvent(ToastRequested{Message: "Alias added."}, false)
}

func (service *EmailAliasesService) showRemoved(list AliasList, failure error) {
	if failure != nil {
		service.AliasError = "Could not remove alias."
		return
	}
	service.Aliases = list.Aliases
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
