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

func (service *AccountAdminService) Create(username string, password string) (string, error) {
	id, failure := service.users().Create(keycloak.NewUser{Username: username, Password: password})
	return id, service.translated(failure)
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

func (service *AccountAdminService) Delete(personId string) error {
	return service.translated(service.users().Delete(personId))
}

func (service *AccountAdminService) IsOperator(personId string) (bool, error) {
	operator, failure := service.users().IsOperator(personId)
	return operator, service.translated(failure)
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
		return &Duplicate{Message: "this username is already taken"}
	case errors.As(failure, &keycloak.UserMissing{}):
		return &NotFound{Message: "no such account"}
	}
	return &Unavailable{Message: "the identity server is not available", Cause: failure}
}

func (service *AccountAdminService) timeout() time.Duration {
	return 30 * time.Second
}
