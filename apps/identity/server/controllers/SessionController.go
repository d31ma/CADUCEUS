import (
	"errors"

	"mesail.dev/packages/oidc"
)

// @RequestRoute("/api/v1/identity/session")
// @Public
type SessionController struct {
}

// @PostRoute
func (controller *SessionController) PostSession(
	// @FromBody
	request SignInRequest,
) (Session, error) {
	grant, failed := oidc.SessionIssuerFactory{}.Configured().SignIn(request.Address, request.Password, request.OneTimeCode)
	return controller.session(grant), controller.failure(failed)
}

// @PostRoute("/refresh")
func (controller *SessionController) PostRefresh(
	// @FromBody
	request RefreshRequest,
) (Session, error) {
	grant, failed := oidc.SessionIssuerFactory{}.Configured().Refresh(request.SessionKey)
	return controller.session(grant), controller.failure(failed)
}

// @DeleteRoute
// @ResponseStatus(204)
func (controller *SessionController) DeleteSession(
	// @FromBody
	request RefreshRequest,
) error {
	return controller.failure(oidc.SessionIssuerFactory{}.Configured().SignOut(request.SessionKey))
}

func (controller *SessionController) session(grant oidc.SessionGrant) Session {
	caller := grant.Caller
	me := Me{MemberId: caller.MemberId, DomainId: caller.DomainId, DisplayName: caller.DisplayName, PrimaryAddress: caller.Email, Roles: caller.Roles}
	return Session{Me: me, AccessToken: grant.AccessToken, ExpiresIn: grant.ExpiresIn, SessionKey: grant.SessionKey}
}

func (controller *SessionController) failure(failed error) error {
	switch {
	case failed == nil:
		return nil
	case errors.Is(failed, oidc.AccountLocked{}):
		return &RateLimited{}
	case errors.Is(failed, oidc.SessionUnavailable{}):
		return &Unavailable{}
	}
	return &Unauthenticated{}
}
