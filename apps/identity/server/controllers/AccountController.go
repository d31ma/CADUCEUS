import (
	"errors"

	"mesail.dev/packages/oidc"
)

// @RequestRoute("/api/v1/identity")
// @SignedIn
type AccountController struct {
}

// @PostRoute("/session")
// @Public
func (controller *AccountController) PostSession(
	// @FromBody
	request SignInRequest,
) (Session, error) {
	grant, failed := oidc.SessionIssuerFactory{}.Configured().SignIn(request.Address, request.Password, request.OneTimeCode)
	return controller.session(grant), controller.failure(failed)
}

// @PostRoute("/session/refresh")
// @Public
func (controller *AccountController) PostRefresh(
	// @FromBody
	request RefreshRequest,
) (Session, error) {
	grant, failed := oidc.SessionIssuerFactory{}.Configured().Refresh(request.SessionKey)
	return controller.session(grant), controller.failure(failed)
}

// @DeleteRoute("/session")
// @Public
// @ResponseStatus(204)
func (controller *AccountController) DeleteSession(
	// @FromBody
	request RefreshRequest,
) error {
	return controller.failure(oidc.SessionIssuerFactory{}.Configured().SignOut(request.SessionKey))
}

// @PostRoute("/password")
func (controller *AccountController) PostPassword(
	// @FromHeader("Authorization")
	authorization string,
	// @FromBody
	request PasswordChange,
) (Session, error) {
	grant, failed := oidc.AccountManagerFactory{}.Configured().ChangePassword(authorization, request.CurrentPassword, request.NewPassword, request.OneTimeCode)
	return controller.session(grant), controller.failure(failed)
}

// @GetRoute("/totp")
func (controller *AccountController) GetTotp(
	// @FromHeader("Authorization")
	authorization string,
) (TotpStatus, error) {
	enabled, failed := oidc.AccountManagerFactory{}.Configured().TotpEnabled(authorization)
	return TotpStatus{Enabled: enabled}, controller.failure(failed)
}

// @PostRoute("/totp/setup")
func (controller *AccountController) PostTotpSetup(
	// @FromHeader("Authorization")
	authorization string,
	// @FromBody
	request TotpSetupRequest,
) (TotpSetup, error) {
	setup, failed := oidc.AccountManagerFactory{}.Configured().SetupTotp(authorization, request.CurrentPassword)
	return TotpSetup{Secret: setup.Secret, OtpauthUri: setup.Uri, QrImage: setup.QrImage, SetupKey: setup.SetupKey}, controller.failure(failed)
}

// @PostRoute("/totp/confirm")
// @ResponseStatus(204)
func (controller *AccountController) PostTotpConfirm(
	// @FromHeader("Authorization")
	authorization string,
	// @FromBody
	request TotpConfirm,
) error {
	return controller.failure(oidc.AccountManagerFactory{}.Configured().ConfirmTotp(authorization, request.SetupKey, request.Code))
}

// @DeleteRoute("/totp")
// @ResponseStatus(204)
func (controller *AccountController) DeleteTotp(
	// @FromHeader("Authorization")
	authorization string,
	// @FromBody
	request TotpRemove,
) error {
	return controller.failure(oidc.AccountManagerFactory{}.Configured().RemoveTotp(authorization, request.CurrentPassword, request.OneTimeCode))
}

func (controller *AccountController) session(grant oidc.SessionGrant) Session {
	caller := grant.Caller
	me := Me{MemberId: caller.MemberId, DomainId: caller.DomainId, DisplayName: caller.DisplayName, PrimaryAddress: caller.Email, Roles: caller.Roles}
	return Session{Me: me, AccessToken: grant.AccessToken, ExpiresIn: grant.ExpiresIn, SessionKey: grant.SessionKey}
}

func (controller *AccountController) failure(failed error) error {
	refused := oidc.AdminRefused{}
	switch {
	case failed == nil:
		return nil
	case errors.As(failed, &refused):
		return &PasswordPolicy{Rule: refused.Message}
	case errors.Is(failed, oidc.InvalidCode{}):
		return &InvalidCode{}
	case errors.Is(failed, oidc.AccountLocked{}):
		return &RateLimited{}
	case errors.Is(failed, oidc.SessionUnavailable{}):
		return &Unavailable{}
	}
	return &Unauthenticated{}
}
