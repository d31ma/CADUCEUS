import (
	"cmp"
	"errors"

	"mesail.dev/packages/login"
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
	grant, failed := login.SessionsFactory{}.Configured().SignIn(cmp.Or(request.Login, request.Address), request.Password, request.OneTimeCode)
	return controller.session(grant), controller.failure(failed)
}

// @PostRoute("/session/refresh")
// @Public
func (controller *AccountController) PostRefresh(
	// @FromBody
	request RefreshRequest,
) (Session, error) {
	grant, failed := login.SessionsFactory{}.Configured().Refresh(request.SessionKey)
	return controller.session(grant), controller.failure(failed)
}

// @PostRoute("/session/domain")
// @Public
func (controller *AccountController) PostDomain(
	// @FromBody
	request DomainSwitch,
) (Session, error) {
	grant, failed := login.SessionsFactory{}.Configured().Switch(request.SessionKey, request.DomainId)
	return controller.session(grant), controller.failure(failed)
}

// @DeleteRoute("/session")
// @Public
// @ResponseStatus(204)
func (controller *AccountController) DeleteSession(
	// @FromBody
	request RefreshRequest,
) error {
	return controller.failure(login.SessionsFactory{}.Configured().SignOut(request.SessionKey))
}

// @PostRoute("/password")
func (controller *AccountController) PostPassword(
	// @FromHeader("Authorization")
	authorization string,
	// @FromBody
	request PasswordChange,
) (Session, error) {
	grant, failed := login.SessionsFactory{}.Configured().ChangePassword(authorization, request.CurrentPassword, request.NewPassword, request.OneTimeCode)
	return controller.session(grant), controller.failure(failed)
}

// @GetRoute("/totp")
func (controller *AccountController) GetTotp(
	// @FromHeader("Authorization")
	authorization string,
) (TotpStatus, error) {
	enabled, failed := login.SessionsFactory{}.Configured().TotpEnabled(authorization)
	return TotpStatus{Enabled: enabled}, controller.failure(failed)
}

// @PostRoute("/totp/setup")
func (controller *AccountController) PostTotpSetup(
	// @FromHeader("Authorization")
	authorization string,
	// @FromBody
	request TotpSetupRequest,
) (TotpSetup, error) {
	setup, failed := login.SessionsFactory{}.Configured().SetupTotp(authorization, request.CurrentPassword)
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
	return controller.failure(login.SessionsFactory{}.Configured().ConfirmTotp(authorization, request.SetupKey, request.Code))
}

// @DeleteRoute("/totp")
// @ResponseStatus(204)
func (controller *AccountController) DeleteTotp(
	// @FromHeader("Authorization")
	authorization string,
	// @FromBody
	request TotpRemove,
) error {
	return controller.failure(login.SessionsFactory{}.Configured().RemoveTotp(authorization, request.CurrentPassword, request.OneTimeCode))
}

func (controller *AccountController) session(grant login.Grant) Session {
	me := Me{MemberId: grant.PersonId, DomainId: grant.DomainId, Username: grant.Username, DisplayName: grant.DisplayName, PrimaryAddress: grant.PrimaryAddress, Roles: grant.Roles}
	return Session{Me: me, Domains: controller.domains(grant.Domains), AccessToken: grant.AccessToken, ExpiresIn: grant.ExpiresIn, SessionKey: grant.SessionKey, MustChangePassword: grant.MustChangePassword}
}

func (controller *AccountController) domains(choices []login.DomainChoice) []DomainChoice {
	domains := []DomainChoice{}
	for _, choice := range choices {
		domains = append(domains, DomainChoice{DomainId: choice.DomainId, Admin: choice.Admin, Default: choice.Default})
	}
	return domains
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
	case errors.Is(failed, login.NotAMember{}):
		return &Forbidden{Code: "not_a_member"}
	case errors.Is(failed, login.NoDomain{}):
		return &Forbidden{Code: "no_domain"}
	case errors.Is(failed, login.AccessRevoked{}):
		return &Forbidden{Code: "access_revoked"}
	case errors.Is(failed, login.NotFound{}):
		return &NotFound{}
	}
	return &Unauthenticated{}
}
