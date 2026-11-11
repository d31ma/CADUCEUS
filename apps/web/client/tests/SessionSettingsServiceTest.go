// @Test(SessionTestSetup, {"partition":"tests"})
type SessionSettingsServiceTest struct {
	behaviors *BehaviorContext
}

func (test *SessionSettingsServiceTest) settings() *SessionSettingsService {
	BrowserFakes{}.Answer("clock", "now", 1000)
	client := NewIdentityClientWith(test.behaviors)
	session := &SessionService{
		realm:    &SessionRealmService{},
		tokens:   &SessionTokenService{identity: client},
		accounts: &SessionAccountService{},
		problems: &SessionProblemService{},
	}
	session.Restore(SessionRestoreRequested{})
	session.Email = "ana@example.com"
	session.Password = "right"
	session.SignIn()
	return &SessionSettingsService{session: session, identity: client, problems: &SessionProblemService{}}
}

func (test *SessionSettingsServiceTest) TestStartPasswordChangeOpensTheForm() {
	settings := test.settings()
	settings.StartPasswordChange()
	Assert.Equals("password", settings.Step)
}

func (test *SessionSettingsServiceTest) TestChangePasswordConfirmsAndClearsTheForm() {
	settings := test.settings()
	settings.StartPasswordChange()
	settings.CurrentPassword = "right"
	settings.NewPassword = "longer-and-better"
	settings.ChangePassword()
	Assert.Equals("", settings.Problem)
	Assert.Equals("passwordChanged", settings.Message)
	Assert.Equals("", settings.Step)
	Assert.Equals("", settings.NewPassword)
}

func (test *SessionSettingsServiceTest) TestChangePasswordRefusedKeepsTheForm() {
	settings := test.settings()
	settings.StartPasswordChange()
	settings.CurrentPassword = "right"
	settings.NewPassword = "weak"
	settings.ChangePassword()
	Assert.Equals("unavailable", settings.Problem)
	Assert.Equals("password", settings.Step)
	Assert.Equals("", settings.CurrentPassword)
}

func (test *SessionSettingsServiceTest) TestSetUpTwoStepShowsTheSecret() {
	settings := test.settings()
	settings.StartTwoStep()
	settings.CurrentPassword = "right"
	settings.SetUpTwoStep()
	Assert.Equals("twoStepConfirm", settings.Step)
	Assert.Equals("JBSWY3DPEHPK3PXP", settings.TwoStepSecret)
	Assert.Equals("data:image/png;base64,AAAA", settings.TwoStepQrImage)
}

func (test *SessionSettingsServiceTest) TestSetUpTwoStepRefusesAWrongPassword() {
	settings := test.settings()
	settings.StartTwoStep()
	settings.CurrentPassword = "wrong"
	settings.SetUpTwoStep()
	Assert.Equals("twoStepPassword", settings.Step)
	Assert.Equals("", settings.TwoStepSecret)
}

func (test *SessionSettingsServiceTest) TestConfirmTwoStepTurnsItOn() {
	settings := test.settings()
	settings.StartTwoStep()
	settings.CurrentPassword = "right"
	settings.SetUpTwoStep()
	settings.Code = "123456"
	settings.ConfirmTwoStep()
	Assert.Equals("twoStepOn", settings.Message)
	Assert.Equals("", settings.TwoStepSecret)
}

func (test *SessionSettingsServiceTest) TestConfirmTwoStepRefusesAWrongCode() {
	settings := test.settings()
	settings.StartTwoStep()
	settings.CurrentPassword = "right"
	settings.SetUpTwoStep()
	settings.Code = "000000"
	settings.ConfirmTwoStep()
	Assert.Equals("twoStepConfirm", settings.Step)
	Assert.Equals("", settings.Code)
	Assert.Equals("JBSWY3DPEHPK3PXP", settings.TwoStepSecret)
}

func (test *SessionSettingsServiceTest) TestRemoveTwoStepTurnsItOff() {
	settings := test.settings()
	settings.StartTwoStepRemoval()
	settings.CurrentPassword = "right"
	settings.OneTimeCode = "123456"
	settings.RemoveTwoStep()
	Assert.Equals("twoStepOff", settings.Message)
	Assert.Equals("", settings.Step)
}

func (test *SessionSettingsServiceTest) TestRemoveTwoStepRefusedKeepsTheForm() {
	settings := test.settings()
	settings.StartTwoStepRemoval()
	settings.CurrentPassword = "right"
	settings.OneTimeCode = "999999"
	settings.RemoveTwoStep()
	Assert.Equals("twoStepRemove", settings.Step)
}

func (test *SessionSettingsServiceTest) TestCancelClosesTheForm() {
	settings := test.settings()
	settings.StartPasswordChange()
	settings.CurrentPassword = "right"
	settings.Cancel()
	Assert.Equals("", settings.Step)
	Assert.Equals("", settings.CurrentPassword)
}

func (test *SessionSettingsServiceTest) TestLoadTwoStepReadsTheStatus() {
	settings := test.settings()
	settings.LoadTwoStep(SessionRestored{SignedIn: true})
	Assert.Equals("on", settings.TwoStep)
}

func (test *SessionSettingsServiceTest) TestLoadTwoStepWithoutSessionIsUnknown() {
	settings := test.settings()
	settings.session.SignOutAll()
	settings.LoadTwoStep(SessionRestored{SignedIn: false})
	Assert.Equals("unknown", settings.TwoStep)
}

func (test *SessionSettingsServiceTest) TestTurningOnMarksTwoStepOn() {
	settings := test.settings()
	settings.StartTwoStep()
	settings.CurrentPassword = "right"
	settings.SetUpTwoStep()
	settings.Code = "123456"
	settings.ConfirmTwoStep()
	Assert.Equals("on", settings.TwoStep)
}

func (test *SessionSettingsServiceTest) TestTurningOffMarksTwoStepOff() {
	settings := test.settings()
	settings.StartTwoStepRemoval()
	settings.CurrentPassword = "right"
	settings.OneTimeCode = "123456"
	settings.RemoveTwoStep()
	Assert.Equals("off", settings.TwoStep)
}
