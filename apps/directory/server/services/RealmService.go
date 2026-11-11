import (
	"cmp"
	"errors"
	"time"

	"mesail.dev/packages/keycloak"
	"mesail.dev/packages/transport"
)

// @Service
type RealmService struct {
	// @Needs
	settings *SettingsService
}

func (service *RealmService) Create(domainId string) error {
	return service.unavailable(service.adopted(service.admin().CreateRealm(domainId)))
}

func (service *RealmService) Delete(domainId string) error {
	failure := service.admin().DeleteRealm(domainId)
	return cmp.Or(service.notManaged(failure), service.unavailable(failure))
}

func (service *RealmService) admin() keycloak.Admin {
	settings := service.settings.Settings()
	adminSettings := keycloak.AdminSettings{BaseUrl: settings.KeycloakUrl, ClientId: settings.Admin.Client, ClientSecret: settings.Admin.Secret, Secrets: service.secrets(settings.RealmSecrets)}
	return keycloak.NewAdmin(adminSettings, transport.SingleUseTransport{}.Client(service.timeout()))
}

func (service *RealmService) secrets(secrets DirectoryRealmSecrets) keycloak.RealmSecrets {
	return keycloak.RealmSecrets{Identity: secrets.Identity, Service: secrets.Service, Account: secrets.Account}
}

func (service *RealmService) adopted(failure error) error {
	if errors.As(failure, &keycloak.RealmExists{}) {
		return nil
	}
	return failure
}

func (service *RealmService) notManaged(failure error) error {
	if errors.As(failure, &keycloak.RealmNotManaged{}) {
		return &RealmNotManaged{Message: failure.Error()}
	}
	return nil
}

func (service *RealmService) unavailable(failure error) error {
	if failure == nil {
		return nil
	}
	return &Unavailable{Message: "the identity server is not available", Cause: failure}
}

func (service *RealmService) timeout() time.Duration {
	return 30 * time.Second
}
