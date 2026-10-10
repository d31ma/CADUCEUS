// LoginServiceTest covers the login form's checks that run before any
// request.
// @Test(ShellTestSetup, {"partition":"tests"})
type LoginServiceTest struct {
	behaviors *BehaviorContext
}

func (test *LoginServiceTest) login() *LoginService {
	login := &LoginService{}
	login.Start()
	return login
}

func (test *LoginServiceTest) TestAnEmailIsRequired() {
	login := test.login()
	login.RequestMfa()
	Assert.Equals("Email is required.", login.Error)
	Assert.Equals("start", login.Step)
}

func (test *LoginServiceTest) TestFactorsNeedAnOpenTransaction() {
	login := test.login()
	login.VerifyPassword()
	Assert.Equals("Start again from your email address.", login.Error)
	login.VerifyRecoveryCode()
	Assert.Equals("Start again from your email address.", login.Error)
}

func (test *LoginServiceTest) TestTheAuthenticatorCodeHasSixDigits() {
	login := test.login()
	login.TotpCode = "12a456"
	login.ConfirmTotp()
	Assert.Equals("Enter the 6-digit code from your authenticator app.", login.Error)
}

func (test *LoginServiceTest) TestRecoveryStepsSwitchAndGoBack() {
	login := test.login()
	login.UseRecoveryCode()
	Assert.Equals("recovery", login.Step)
	login.ToggleRecoveryMethod()
	Assert.Equals("admin", login.RecoveryMethod)
	login.RecoveryCode = "abc"
	login.Back()
	Assert.Equals("start", login.Step)
	Assert.Equals("", login.RecoveryCode)
}

func (test *LoginServiceTest) TestPasswordSetupChecksTheFields() {
	login := test.login()
	login.SetupPassword()
	Assert.Equals("Password is required.", login.Error)
	login.Password = "one"
	login.ConfirmPassword = "two"
	login.SetupPassword()
	Assert.Equals("Passwords do not match.", login.Error)
}

func (test *LoginServiceTest) TestACancelledPasskeyStaysOnThePasskeyStep() {
	login := test.login()
	login.HearFeatures(BrowserFeatures{Passkeys: true})
	login.HearSession(SessionChanged{Token: "t"})
	login.HearPasskey(PasskeyAnswered{Mode: "get", Failure: "cancelled"})
	Assert.Equals("Passkey authentication was cancelled.", login.Error)
	Assert.Equals("passkey", login.Step)
}

func (test *LoginServiceTest) TestTheAdministratorCodeNeedsAnAddress() {
	login := test.login()
	login.VerifyAdminRecovery()
	Assert.Equals("Email is required.", login.Error)
}
