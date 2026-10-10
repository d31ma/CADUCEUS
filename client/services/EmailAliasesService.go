import "strings"

// @Service
type EmailAliasesService struct {
	// @Needs
	usersClient               *UsersClient
	applicationEventPublisher ApplicationEventPublisher
	authorization             string
	PrimaryEmail              string
	Aliases                   []string
	NewAlias                  string
	AliasError                string
	AliasSaving               bool
	Loading                   bool
}

// @OnEvent({"replay":true})
func (service *EmailAliasesService) Open(session AccountSession) error {
	service.Loading = true
	service.authorization = "Bearer " + session.Token
	service.announce(true)
	list, failure := service.usersClient.Aliases(service.authorization)
	service.showLoaded(list, failure)
	service.announce(false)
	return nil
}

func (service *EmailAliasesService) Add() error {
	alias := strings.TrimSpace(service.NewAlias)
	service.AliasSaving = true
	service.AliasError = ""
	service.announce(true)
	list, failure := service.usersClient.AddAlias(service.authorization, service.aliasBody(alias))
	service.showAdded(list, failure)
	service.announce(false)
	return nil
}

func (service *EmailAliasesService) Remove(alias string) error {
	list, failure := service.usersClient.RemoveAlias(service.authorization, service.aliasBody(alias))
	service.showRemoved(list, failure)
	return nil
}

func (service *EmailAliasesService) showLoaded(list AliasList, failure error) {
	service.Loading = false
	if failure != nil {
		service.Aliases = []string{}
		return
	}
	service.PrimaryEmail = list.Email
	service.Aliases = list.Aliases
}

func (service *EmailAliasesService) showAdded(list AliasList, failure error) {
	service.AliasSaving = false
	if failure != nil {
		service.AliasError = "Could not add alias."
		return
	}
	service.PrimaryEmail = list.Email
	service.Aliases = list.Aliases
	service.NewAlias = ""
	service.applicationEventPublisher.PublishEvent(ToastRequested{Message: "Alias added."}, false)
}

func (service *EmailAliasesService) showRemoved(list AliasList, failure error) {
	if failure != nil {
		service.AliasError = "Could not remove alias."
		return
	}
	service.Aliases = list.Aliases
	service.applicationEventPublisher.PublishEvent(ToastRequested{Message: "Alias removed."}, false)
}

func (service *EmailAliasesService) aliasBody(alias string) BehaviorValue {
	return map[string]any{"alias": alias}
}

func (service *EmailAliasesService) announce(busy bool) {
	service.applicationEventPublisher.PublishEvent(SettingsActivity{Section: "aliases", Busy: busy}, false)
}
