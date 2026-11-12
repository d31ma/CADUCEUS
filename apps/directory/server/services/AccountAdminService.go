import (
	"cmp"
	"errors"
	"time"

	"mesail.dev/packages/keycloak"
	"mesail.dev/packages/transport"
)

// @Service
type AccountAdminService struct {
	// @Needs
	settings *SettingsService
}

func (service *AccountAdminService) Create(username string, password string, addRequest string) (string, error) {
	user := keycloak.NewUser{Username: username, Password: password, AddRequest: addRequest}
	id, failure := service.users().Create(user)
	taken := errors.As(failure, &keycloak.UserExists{})
	return service.recoveredWhenTaken(taken, id, service.translated(failure), user)
}

func (service *AccountAdminService) SetEnabled(personId string, enabled bool) error {
	return service.translated(service.users().SetEnabled(personId, enabled))
}

func (service *AccountAdminService) ResetPassword(personId string, password string) error {
	return service.translated(service.users().ResetPassword(personId, password))
}

func (service *AccountAdminService) RemoveTotp(personId string) error {
	return service.translated(service.users().RemoveTotp(personId))
}

func (service *AccountAdminService) RemovePasskeys(personId string) error {
	return service.translated(service.users().RemovePasskeys(personId))
}

func (service *AccountAdminService) Delete(personId string) error {
	return service.translated(service.users().Delete(personId))
}

func (service *AccountAdminService) IsOperator(personId string) (bool, error) {
	operator, failure := service.users().IsOperator(personId)
	return operator, service.translated(failure)
}

func (service *AccountAdminService) recoveredWhenTaken(taken bool, id string, translated error, user keycloak.NewUser) (string, error) {
	if !taken {
		return id, translated
	}
	return service.recovered(user)
}

func (service *AccountAdminService) recovered(user keycloak.NewUser) (string, error) {
	tagged, failure := service.users().Tagged(user.Username)
	missing := errors.As(failure, &keycloak.UserMissing{})
	refused := service.unlessMissing(missing, service.translated(failure))
	refusal := service.madeByThisAdd(refused, service.sameTag(tagged.AddRequest, user.AddRequest))
	return tagged.Id, Steps{}.Unless(refusal, func() error { return service.ResetPassword(tagged.Id, user.Password) })
}

func (service *AccountAdminService) unlessMissing(missing bool, failure error) error {
	if missing {
		return nil
	}
	return failure
}

func (service *AccountAdminService) madeByThisAdd(refused error, sameTag bool) error {
	switch {
	case refused != nil:
		return refused
	case sameTag:
		return nil
	}
	return &Duplicate{Message: service.taken()}
}

func (service *AccountAdminService) sameTag(tag string, addRequest string) bool {
	return tag != "" && tag == addRequest
}

func (service *AccountAdminService) taken() string {
	return "this username is already taken"
}

func (service *AccountAdminService) users() keycloak.Users {
	settings := service.settings.Settings()
	userSettings := keycloak.UserSettings{BaseUrl: settings.KeycloakUrl, ClientId: settings.Admin.Client, ClientSecret: settings.Admin.Secret}
	return keycloak.NewUsers(userSettings, transport.SingleUseTransport{}.Client(service.timeout()))
}

func (service *AccountAdminService) translated(failure error) error {
	refused := keycloak.PasswordRefused{}
	switch {
	case failure == nil:
		return nil
	case errors.As(failure, &refused):
		return &PasswordPolicy{Message: cmp.Or(refused.Message, "the password does not meet the password policy")}
	case errors.As(failure, &keycloak.UserExists{}):
		return &Duplicate{Message: service.taken()}
	case errors.As(failure, &keycloak.UserMissing{}):
		return &NotFound{Message: "no such account"}
	}
	return &Unavailable{Message: "the identity server is not available", Cause: failure}
}

func (service *AccountAdminService) timeout() time.Duration {
	return 30 * time.Second
}
