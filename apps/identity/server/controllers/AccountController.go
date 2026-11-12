import (
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
	grant, failed := login.SessionsFactory{}.Configured().SignIn(request.Login, request.Password, request.OneTimeCode)
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

// @PostRoute("/passkeys/challenge")
// @Public
func (controller *AccountController) PostPasskeyChallenge() (PasskeyChallenge, error) {
	challenge, failed := login.SessionsFactory{}.Configured().PasskeyChallenge()
	return PasskeyChallenge{OptionsJson: challenge.OptionsJson, ChallengeKey: challenge.ChallengeKey}, controller.failure(failed)
}

// @PostRoute("/session/passkey")
// @Public
func (controller *AccountController) PostPasskeySession(
	// @FromBody
	request PasskeySignIn,
) (Session, error) {
	grant, failed := login.SessionsFactory{}.Configured().PasskeySignIn(request.ChallengeKey, request.CredentialJson, request.DomainId)
	return controller.session(grant), controller.failure(failed)
}

// @GetRoute("/passkeys")
func (controller *AccountController) GetPasskeys(
	// @FromHeader("Authorization")
	authorization string,
) ([]Passkey, error) {
	passkeys, failed := login.SessionsFactory{}.Configured().Passkeys(authorization)
	return controller.passkeys(passkeys), controller.failure(failed)
}

// @PostRoute("/passkeys/setup")
func (controller *AccountController) PostPasskeySetup(
	// @FromHeader("Authorization")
	authorization string,
	// @FromBody
	request PasskeySetupRequest,
) (PasskeySetup, error) {
	setup, failed := login.SessionsFactory{}.Configured().SetupPasskey(authorization, request.CurrentPassword, request.OneTimeCode)
	return PasskeySetup{OptionsJson: setup.OptionsJson, SetupKey: setup.SetupKey}, controller.failure(failed)
}

// @PostRoute("/passkeys")
// @ResponseStatus(201)
func (controller *AccountController) PostPasskey(
	// @FromHeader("Authorization")
	authorization string,
	// @FromBody
	request PasskeyConfirm,
) (Passkey, error) {
	added, failed := login.SessionsFactory{}.Configured().ConfirmPasskey(authorization, request.SetupKey, request.CredentialJson, request.Label)
	return controller.passkey(added), controller.failure(failed)
}

// @PatchRoute("/passkeys/{passkey_id}")
func (controller *AccountController) PatchPasskey(
	// @FromHeader("Authorization")
	authorization string,
	// @FromPath("passkey_id")
	passkeyId string,
	// @FromBody
	request PasskeyRename,
) (Passkey, error) {
	renamed, failed := login.SessionsFactory{}.Configured().RenamePasskey(authorization, passkeyId, request.Label)
	return controller.passkey(renamed), controller.failure(failed)
}

// @DeleteRoute("/passkeys/{passkey_id}")
func (controller *AccountController) DeletePasskey(
	// @FromHeader("Authorization")
	authorization string,
	// @FromPath("passkey_id")
	passkeyId string,
	// @FromBody
	request PasskeyRemove,
) (Session, error) {
	removal := login.PasskeyRemoval{CurrentPassword: request.CurrentPassword, OneTimeCode: request.OneTimeCode}
	grant, failed := login.SessionsFactory{}.Configured().RemovePasskey(authorization, passkeyId, removal)
	return controller.session(grant), controller.failure(failed)
}

func (controller *AccountController) passkeys(list []oidc.Passkey) []Passkey {
	passkeys := []Passkey{}
	for _, item := range list {
		passkeys = append(passkeys, controller.passkey(item))
	}
	return passkeys
}

func (controller *AccountController) passkey(item oidc.Passkey) Passkey {
	return Passkey{PasskeyId: item.PasskeyId, Label: item.Label, CreatedAt: item.CreatedAt, LastUsedAt: item.LastUsedAt}
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
	case errors.Is(failed, oidc.SetupExpired{}):
		return &BadPasskey{Code: oidc.SetupExpired{}.PasskeyCode()}
	case errors.Is(failed, oidc.InvalidPasskey{}):
		return &BadPasskey{Code: oidc.InvalidPasskey{}.PasskeyCode()}
	case errors.Is(failed, oidc.TooManyPasskeys{}):
		return &Conflict{Code: oidc.TooManyPasskeys{}.PasskeyCode()}
	case errors.Is(failed, oidc.LabelExists{}):
		return &Conflict{Code: oidc.LabelExists{}.PasskeyCode()}
	case errors.Is(failed, oidc.InvalidLabel{}):
		return &InvalidLabel{}
	case errors.Is(failed, oidc.PasskeyNotFound{}):
		return &NotFound{}
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
