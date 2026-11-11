import (
	"net/http"

	"mesail.dev/packages/oidc"
)

// @RequestRoute("/api/v1/identity")
type IdentityController struct {
}

// @GetRoute("/me")
// @SignedIn
func (controller *IdentityController) GetMe(request *http.Request) (Me, error) {
	caller, refused := oidc.Configured().Identify(request.Header.Get("Authorization"))
	return controller.me(caller), controller.authenticated(refused)
}

func (controller *IdentityController) me(caller oidc.Caller) Me {
	return Me{MemberId: caller.MemberId, DomainId: caller.DomainId, DisplayName: caller.DisplayName, PrimaryAddress: caller.Email, Roles: caller.Roles}
}

func (controller *IdentityController) authenticated(refused error) error {
	if refused != nil {
		return &Unauthenticated{}
	}
	return nil
}
