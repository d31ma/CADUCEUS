import (
	"net/http"

	"mesail.dev/packages/login"
	"mesail.dev/packages/oidc"
)

// @Service
type MeService struct {
}

func (service *MeService) Me(request *http.Request) (Me, error) {
	caller, refused := login.SessionsFactory{}.Configured().Identify(request.Header.Get("Authorization"))
	return service.me(caller), service.authenticated(refused)
}

func (service *MeService) me(caller oidc.Caller) Me {
	return Me{MemberId: caller.MemberId, DomainId: caller.DomainId, Username: caller.Username, DisplayName: caller.DisplayName, PrimaryAddress: caller.Email, Roles: caller.Roles}
}

func (service *MeService) authenticated(refused error) error {
	if refused != nil {
		return Unauthenticated{}.TokenRefused()
	}
	return nil
}
