import (
	"regexp"
	"slices"
)

// LoginService drives one sign-in transaction for the login form. The server
// decides which factors the deployment requires; the form only answers them.
// Passkey ceremonies run in PasskeyService, which owns navigator.credentials.
// The browser transport hands over no error body, so a refused request shows
// the form's own fixed message instead of the server's text.
// @Service
type LoginService struct {
	// @Needs
	authClient                *AuthClient
	applicationEventPublisher ApplicationEventPublisher
	// @SessionState
	Step string
	// @SessionState
	Email string
	// @SessionState
	TotpCode string
	// @SessionState
	ConfirmPassword string
	// @SessionState
	RecoveryCode string
	// @SessionState
	RecoveryMethod string
	// @SessionState
	Loading bool
	// @SessionState
	PrimarySatisfied bool
	Password         string
	Error            string
	transactionID    string
	requiredFactors  []string
	satisfied        bool
	passkeys         bool
	token            string
	stepUpID         string
}

// @OnStart
func (login *LoginService) Start() {
	login.Step = TextUtils{}.FirstOf(login.Step, login.steps().Start)
	login.RecoveryMethod = TextUtils{}.FirstOf(login.RecoveryMethod, login.steps().Saved)
}

// HearFeatures learns whether this browser has passkeys.
// @OnEvent({"replay":true})
func (login *LoginService) HearFeatures(features BrowserFeatures) {
	login.passkeys = features.Passkeys
}

// HearSession keeps the access token that signed-in credential changes carry.
// @OnEvent({"replay":true})
func (login *LoginService) HearSession(changed SessionChanged) {
	login.token = changed.Token
}

// RequestMfa submits the address and opens a transaction. The answer never
// tells an unknown address from a known one.
func (login *LoginService) RequestMfa() {
	login.require(login.Email != "", "Email is required.", login.begin)
}

// VerifyPassword answers the password factor.
func (login *LoginService) VerifyPassword() {
	login.require(login.transactionID != "", login.startAgain(), login.passwordGiven)
}

// ConfirmTotp answers the authenticator-app factor.
func (login *LoginService) ConfirmTotp() {
	login.require(login.sixDigits(login.TotpCode), "Enter the 6-digit code from your authenticator app.", login.authenticatorCode)
}

// UseRecoveryCode shows the saved-recovery-code step.
func (login *LoginService) UseRecoveryCode() {
	login.recoveryWith(login.steps().Saved)
}

// ToggleRecoveryMethod switches between the two kinds of recovery code.
func (login *LoginService) ToggleRecoveryMethod() {
	login.recoveryWith(login.otherRecoveryMethod(login.RecoveryMethod))
}

// VerifyRecoveryCode spends a single-use recovery code as the second factor.
func (login *LoginService) VerifyRecoveryCode() {
	login.require(login.transactionID != "", login.startAgain(), login.recoveryGiven)
}

// VerifyAdminRecovery spends an administrator code, opening the transaction
// first when the address has not been submitted yet.
func (login *LoginService) VerifyAdminRecovery() {
	login.openWhenMissing(login.transactionID == "")
	login.require(login.transactionID != "", login.Error, login.recoveryGiven)
}

// SetupPassword sets a password on a signed-in session, then enrolls a passkey.
func (login *LoginService) SetupPassword() {
	login.require(login.Password != "", login.passwordRequired(), login.passwordsCompared)
}

// Back returns one step and clears the current error.
func (login *LoginService) Back() {
	login.Error = ""
	login.backFrom(login.Step)
}

// HearPasskey continues the ceremony that PasskeyService finished.
// @OnEvent
func (login *LoginService) HearPasskey(answer PasskeyAnswered) {
	login.Loading = false
	login.passkeyOutcome(answer.Failure, answer.Mode)(answer)
	login.applicationEventPublisher.PublishEvent(LoginProgressed{Step: login.Step, Error: login.Error}, false)
}

func (login *LoginService) begin() {
	login.answer(func() (LoginState, error) { return login.authClient.Begin(LoginStart{Address: login.Email}) },
		"Unable to sign in.", func(state LoginState) {
			login.PrimarySatisfied = false
			login.adopt(state)
		})
}

func (login *LoginService) passwordGiven() {
	login.require(login.Password != "", login.passwordRequired(), login.password)
}

func (login *LoginService) password() {
	login.answer(func() (LoginState, error) {
		factor := PasswordFactor{TransactionID: login.transactionID, Password: login.Password}
		login.Password = ""
		return login.authClient.Password(factor)
	}, "Unable to verify password.", func(state LoginState) {
		login.PrimarySatisfied = true
		login.Step = login.steps().Passkey
		login.adopt(state)
	})
}

func (login *LoginService) authenticatorCode() {
	login.answer(func() (LoginState, error) {
		factor := CodeFactor{TransactionID: login.transactionID, Code: login.TotpCode}
		login.TotpCode = ""
		return login.authClient.AuthenticatorCode(factor)
	}, "Invalid code.", login.adopt)
}

func (login *LoginService) recoveryGiven() {
	login.require(login.RecoveryCode != "", "A recovery code is required.", login.recovery)
}

func (login *LoginService) recovery() {
	login.answer(func() (LoginState, error) {
		factor := CodeFactor{TransactionID: login.transactionID, Code: login.RecoveryCode}
		login.RecoveryCode = ""
		return login.authClient.Recovery(factor)
	}, "Invalid recovery code.", login.adopt)
}

// answer sends one factor while the form shows that it is working.
func (login *LoginService) answer(send func() (LoginState, error), fallback string, next func(LoginState)) {
	login.working()
	state, failure := send()
	login.Loading = false
	login.settle(failure, fallback, func() { next(state) })
}

// adopt takes one reported transaction state and moves to the step that
// satisfies what it still needs. Only a satisfied transaction is completed.
func (login *LoginService) adopt(state LoginState) {
	login.transactionID = TextUtils{}.FirstOf(state.TransactionID, login.transactionID)
	login.requiredFactors = login.factorsOr(state.RequiredFactors, login.requiredFactors)
	login.satisfied = state.Satisfied
	login.nextAfter(state.State == "failed", login.needsPassword())()
}

func (login *LoginService) nextAfter(failed bool, needsPassword bool) func() {
	switch {
	case failed:
		return login.refused
	case login.satisfied:
		return login.complete
	case needsPassword:
		return login.askPassword
	}
	return login.askPasskey
}

func (login *LoginService) refused() {
	login.Error = "Unable to sign in with those details."
	login.Step = login.steps().Start
}

func (login *LoginService) askPassword() {
	login.Step = login.steps().Password
}

func (login *LoginService) askPasskey() {
	login.Step = login.steps().Passkey
	login.require(login.passkeys, login.noPasskeySupport(login.PrimarySatisfied), login.passkeyOptions)
	login.Step = login.fallbackStep(login.passkeys, login.PrimarySatisfied, login.Step)
}

func (login *LoginService) passkeyOptions() {
	login.working()
	options, failure := login.authClient.PasskeyOptions(LoginTransaction{TransactionID: login.transactionID})
	login.Loading = false
	login.settle(failure, "Unable to start passkey sign-in.", func() { login.challenge(options) })
}

func (login *LoginService) challenge(options PasskeyOptions) {
	enrolled := len(options.Credentials) > 0
	login.require(enrolled, login.noPasskeyEnrolled(login.PrimarySatisfied), func() {
		login.Loading = true
		login.applicationEventPublisher.PublishEvent(PasskeyRequested{
			Mode: login.steps().SignIn, Challenge: options.Challenge, RelyingPartyID: options.RelyingPartyID, Credentials: options.Credentials,
		}, false)
	})
	login.Step = login.fallbackStep(enrolled, login.PrimarySatisfied, login.Step)
}

func (login *LoginService) passkeyOutcome(failure string, mode string) func(PasskeyAnswered) {
	switch {
	case failure != "":
		return func(PasskeyAnswered) { login.passkeyFailed(failure, mode) }
	case mode == login.steps().Register:
		return login.saveEnrollment
	}
	return login.assert
}

func (login *LoginService) assert(answer PasskeyAnswered) {
	login.answer(func() (LoginState, error) {
		return login.authClient.Passkey(PasskeyAssertion{
			TransactionID: login.transactionID, CredentialID: answer.CredentialID,
			AuthenticatorData: answer.AuthenticatorData, ClientDataJSON: answer.ClientDataJSON, Signature: answer.Signature,
		})
	}, "Authentication failed.", login.adopt)
}

func (login *LoginService) passkeyFailed(failure string, mode string) {
	login.Error = PasskeyUtils{}.Failure(failure, mode)
	login.Step = login.stepAfterPasskeyFailure(mode, login.Step)
}

func (login *LoginService) complete() {
	tokens, failure := login.authClient.Complete(LoginTransaction{TransactionID: login.transactionID})
	login.settle(failure, "Unable to complete sign-in.", func() {
		login.transactionID = ""
		login.applicationEventPublisher.PublishEvent(tokens, false)
	})
}

func (login *LoginService) passwordsCompared() {
	login.require(login.matchingPasswords(), "Passwords do not match.", login.changePassword)
}

func (login *LoginService) changePassword() {
	login.working()
	login.stepUp(func(stepUp string) {
		_, failure := login.authClient.ChangePassword(login.bearer(), CredentialChange{StepUpTransactionID: stepUp, Password: login.Password})
		login.Password = ""
		login.ConfirmPassword = ""
		login.settle(failure, "Unable to save password.", login.registerPasskey)
	})
	login.Loading = false
}

func (login *LoginService) registerPasskey() {
	login.Step = "passkey-setup"
	login.require(login.passkeys, "Passkeys are not supported on this device.", login.enroll)
	login.Step = login.stepWithoutEnrollment(login.passkeys, login.Step)
}

func (login *LoginService) enroll() {
	login.working()
	login.stepUp(func(stepUp string) {
		enrollment, failure := login.authClient.BeginPasskeyEnrollment(login.bearer(), CredentialChange{StepUpTransactionID: stepUp})
		login.settle(failure, "Registration failed.", func() {
			login.applicationEventPublisher.PublishEvent(PasskeyRequested{
				Mode: login.steps().Register, Challenge: enrollment.Challenge, RelyingPartyID: enrollment.RelyingPartyID,
				UserID: enrollment.PrincipalID, UserName: login.Email,
			}, false)
		})
	})
}

func (login *LoginService) saveEnrollment(answer PasskeyAnswered) {
	_, failure := login.authClient.FinishPasskeyEnrollment(login.bearer(), PasskeyAttestation{
		AttestationObject: answer.AttestationObject, ClientDataJSON: answer.ClientDataJSON,
	})
	login.settle(failure, "Failed to save passkey.", func() {
		login.Error = ""
		login.Step = login.steps().Start
	})
}

// stepUp proves a factor again for a credential change: a fresh transaction
// satisfied by a recovery code.
func (login *LoginService) stepUp(next func(string)) {
	begun, failure := login.authClient.Begin(LoginStart{Address: login.Email})
	login.settle(failure, "Unable to re-authenticate.", func() {
		login.stepUpID = begun.TransactionID
		login.require(login.RecoveryCode != "", "Confirm it is you: enter a recovery code to change credentials.", func() {
			login.confirmStepUp(next)
		})
		login.Step = login.stepWithoutCode(login.RecoveryCode != "", login.Step)
	})
}

func (login *LoginService) confirmStepUp(next func(string)) {
	verified, failure := login.authClient.Recovery(CodeFactor{TransactionID: login.stepUpID, Code: login.RecoveryCode})
	login.settle(failure, login.notConfirmed(), func() {
		login.require(verified.Satisfied, login.notConfirmed(), func() { next(login.stepUpID) })
	})
}

func (login *LoginService) recoveryWith(method string) {
	login.RecoveryMethod = method
	login.RecoveryCode = ""
	login.Error = ""
	login.Step = login.steps().Recovery
}

func (login *LoginService) working() {
	login.Loading = true
	login.Error = ""
}

func (login *LoginService) bearer() string {
	return "Bearer " + login.token
}

func (login *LoginService) require(ready bool, message string, next func()) {
	if !ready {
		login.Error = message
		return
	}
	next()
}

func (login *LoginService) settle(failure error, fallback string, next func()) {
	if failure != nil {
		login.Error = HTTPUtils{}.Message(failure, fallback, "Network error. Check your connection.")
		return
	}
	next()
}

func (login *LoginService) openWhenMissing(missing bool) {
	if missing {
		login.RequestMfa()
	}
}

func (login *LoginService) backFrom(step string) {
	switch step {
	case login.steps().Passkey:
		login.Step = login.steps().Start
	case "totp":
		login.TotpCode = ""
		login.Step = login.steps().Start
	case login.steps().Password:
		login.Password = ""
		login.Step = login.steps().Start
	case login.steps().Recovery:
		login.RecoveryCode = ""
		login.Step = login.steps().Start
	}
}

func (login *LoginService) needsPassword() bool {
	return login.Step == login.steps().Start && slices.Contains(login.requiredFactors, login.steps().Password)
}

func (login *LoginService) matchingPasswords() bool {
	return login.ConfirmPassword == "" || login.ConfirmPassword == login.Password
}

func (LoginService) sixDigits(code string) bool {
	return regexp.MustCompile(`^\d{6}$`).MatchString(code)
}

func (login *LoginService) otherRecoveryMethod(method string) string {
	if method == login.steps().Saved {
		return "admin"
	}
	return login.steps().Saved
}


func (LoginService) factorsOr(factors []string, fallback []string) []string {
	if factors == nil {
		return fallback
	}
	return factors
}

func (LoginService) noPasskeySupport(primarySatisfied bool) string {
	if primarySatisfied {
		return "Passkeys are not supported on this device. Use a recovery code."
	}
	return "Passkeys are not supported on this device. Enter your password to continue."
}

func (LoginService) noPasskeyEnrolled(primarySatisfied bool) string {
	if primarySatisfied {
		return "No passkey is enrolled yet. Use a recovery code to finish signing in."
	}
	return "No passkey is enrolled yet. Enter your password to continue."
}

// fallbackStep is where the form goes when no passkey ceremony can start:
// recovery once the password is proven, otherwise the password.
func (login *LoginService) fallbackStep(available bool, primarySatisfied bool, step string) string {
	switch {
	case available:
		return step
	case primarySatisfied:
		return login.steps().Recovery
	}
	return login.steps().Password
}

func (login *LoginService) stepAfterPasskeyFailure(mode string, step string) string {
	if mode == login.steps().SignIn {
		return login.steps().Passkey
	}
	return step
}

func (login *LoginService) stepWithoutEnrollment(passkeys bool, step string) string {
	if passkeys {
		return step
	}
	return login.steps().Start
}

func (login *LoginService) stepWithoutCode(given bool, step string) string {
	if given {
		return step
	}
	return login.steps().Recovery
}

// steps names the form's steps and the two passkey ceremonies.
func (LoginService) steps() LoginSteps {
	return LoginSteps{Start: "start", Password: "password", Passkey: "passkey", Recovery: "recovery", Saved: "code", SignIn: "get", Register: "create"}
}

func (LoginService) passwordRequired() string { return "Password is required." }

func (LoginService) notConfirmed() string { return "That code did not confirm your identity." }

func (LoginService) startAgain() string { return "Start again from your email address." }
