// @RequestRoute("/api/v1/identity")
// @SignedIn
type AccountController struct {
	// @Needs
	sessions *SessionService
	// @Needs
	totp *TotpService
	// @Needs
	passkeys *PasskeyService
}

// @PostRoute("/session")
// @Public
func (controller *AccountController) PostSession(
	// @FromBody
	request SignInRequest,
) (Session, error) {
	return controller.sessions.SignIn(request)
}

// @PostRoute("/session/refresh")
// @Public
func (controller *AccountController) PostRefresh(
	// @FromBody
	request RefreshRequest,
) (Session, error) {
	return controller.sessions.Refresh(request)
}

// @PostRoute("/session/domain")
// @Public
func (controller *AccountController) PostDomain(
	// @FromBody
	request DomainSwitch,
) (Session, error) {
	return controller.sessions.Switch(request)
}

// @DeleteRoute("/session")
// @Public
// @ResponseStatus(204)
func (controller *AccountController) DeleteSession(
	// @FromBody
	request RefreshRequest,
) error {
	return controller.sessions.SignOut(request)
}

// @DeleteRoute("/sessions")
// @ResponseStatus(204)
func (controller *AccountController) DeleteSessions(
	// @FromHeader("Authorization")
	authorization string,
) error {
	return controller.sessions.SignOutEverywhere(authorization)
}

// @PostRoute("/password")
func (controller *AccountController) PostPassword(
	// @FromHeader("Authorization")
	authorization string,
	// @FromBody
	request PasswordChange,
) (Session, error) {
	return controller.sessions.ChangePassword(authorization, request)
}

// @GetRoute("/totp")
func (controller *AccountController) GetTotp(
	// @FromHeader("Authorization")
	authorization string,
) (TotpStatus, error) {
	return controller.totp.Status(authorization)
}

// @PostRoute("/totp/setup")
func (controller *AccountController) PostTotpSetup(
	// @FromHeader("Authorization")
	authorization string,
	// @FromBody
	request TotpSetupRequest,
) (TotpSetup, error) {
	return controller.totp.Setup(authorization, request)
}

// @PostRoute("/totp/confirm")
// @ResponseStatus(204)
func (controller *AccountController) PostTotpConfirm(
	// @FromHeader("Authorization")
	authorization string,
	// @FromBody
	request TotpConfirm,
) error {
	return controller.totp.Confirm(authorization, request)
}

// @DeleteRoute("/totp")
// @ResponseStatus(204)
func (controller *AccountController) DeleteTotp(
	// @FromHeader("Authorization")
	authorization string,
	// @FromBody
	request TotpRemove,
) error {
	return controller.totp.Remove(authorization, request)
}

// @PostRoute("/passkeys/challenge")
// @Public
func (controller *AccountController) PostPasskeyChallenge() (PasskeyChallenge, error) {
	return controller.passkeys.Challenge()
}

// @PostRoute("/session/passkey")
// @Public
func (controller *AccountController) PostPasskeySession(
	// @FromBody
	request PasskeySignIn,
) (Session, error) {
	return controller.passkeys.SignIn(request)
}

// @GetRoute("/passkeys")
func (controller *AccountController) GetPasskeys(
	// @FromHeader("Authorization")
	authorization string,
) ([]Passkey, error) {
	return controller.passkeys.List(authorization)
}

// @PostRoute("/passkeys/setup")
func (controller *AccountController) PostPasskeySetup(
	// @FromHeader("Authorization")
	authorization string,
	// @FromBody
	request PasskeySetupRequest,
) (PasskeySetup, error) {
	return controller.passkeys.Setup(authorization, request)
}

// @PostRoute("/passkeys")
// @ResponseStatus(201)
func (controller *AccountController) PostPasskey(
	// @FromHeader("Authorization")
	authorization string,
	// @FromBody
	request PasskeyConfirm,
) (Passkey, error) {
	return controller.passkeys.Add(authorization, request)
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
	return controller.passkeys.Rename(authorization, passkeyId, request)
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
	return controller.passkeys.Remove(authorization, passkeyId, request)
}
