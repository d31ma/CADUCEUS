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
	session.signIn()
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
	settings.ChangePasswordWith("right", "longer-and-better", "")
	Assert.Equals("", settings.Problem)
	Assert.Equals("passwordChanged", settings.Notice)
	Assert.Equals("", settings.Step)
	Assert.Equals("", settings.NewPassword)
}

func (test *SessionSettingsServiceTest) TestChangePasswordRefusedKeepsTheForm() {
	settings := test.settings()
	settings.StartPasswordChange()
	settings.ChangePasswordWith("right", "weak", "")
	Assert.Equals("unavailable", settings.Problem)
	Assert.Equals("password", settings.Step)
	Assert.Equals("", settings.CurrentPassword)
}

func (test *SessionSettingsServiceTest) TestSetUpTwoStepShowsTheSecret() {
	settings := test.settings()
	settings.StartTwoStep()
	settings.SetUpTwoStepWith("right", "")
	Assert.Equals("twoStepScan", settings.Step)
	Assert.Equals("JBSWY3DPEHPK3PXP", settings.TwoStepSecret)
	Assert.Equals("data:image/png;base64,AAAA", settings.TwoStepQrImage)
}

func (test *SessionSettingsServiceTest) TestSetUpTwoStepRefusesAWrongPassword() {
	settings := test.settings()
	settings.StartTwoStep()
	settings.SetUpTwoStepWith("wrong", "")
	Assert.Equals("twoStepPassword", settings.Step)
	Assert.Equals("", settings.TwoStepSecret)
}

func (test *SessionSettingsServiceTest) TestConfirmTwoStepTurnsItOn() {
	settings := test.settings()
	settings.StartTwoStep()
	settings.SetUpTwoStepWith("right", "")
	settings.ConfirmTwoStepWith("123456")
	Assert.Equals("twoStepOn", settings.Notice)
	Assert.Equals("", settings.TwoStepSecret)
}

func (test *SessionSettingsServiceTest) TestConfirmTwoStepRefusesAWrongCode() {
	settings := test.settings()
	settings.StartTwoStep()
	settings.SetUpTwoStepWith("right", "")
	settings.ShowCode()
	settings.ConfirmTwoStepWith("000000")
	Assert.Equals("twoStepCode", settings.Step)
	Assert.Equals("", settings.Code)
	Assert.Equals("JBSWY3DPEHPK3PXP", settings.TwoStepSecret)
}

func (test *SessionSettingsServiceTest) TestRemoveTwoStepTurnsItOff() {
	settings := test.settings()
	settings.StartTwoStepRemoval()
	settings.RemoveTwoStepWith("right", "123456")
	Assert.Equals("twoStepOff", settings.Notice)
	Assert.Equals("", settings.Step)
}

func (test *SessionSettingsServiceTest) TestRemoveTwoStepRefusedKeepsTheForm() {
	settings := test.settings()
	settings.StartTwoStepRemoval()
	settings.RemoveTwoStepWith("right", "999999")
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

func (test *SessionSettingsServiceTest) TestRetryTwoStepWithoutSessionIsUnknown() {
	settings := test.settings()
	settings.session.SignOutAll()
	settings.RetryTwoStep()
	Assert.Equals("unknown", settings.TwoStep)
}

func (test *SessionSettingsServiceTest) TestTurningOnMarksTwoStepOn() {
	settings := test.settings()
	settings.StartTwoStep()
	settings.SetUpTwoStepWith("right", "")
	settings.ConfirmTwoStepWith("123456")
	Assert.Equals("on", settings.TwoStep)
}

func (test *SessionSettingsServiceTest) TestTurningOffMarksTwoStepOff() {
	settings := test.settings()
	settings.StartTwoStepRemoval()
	settings.RemoveTwoStepWith("right", "123456")
	Assert.Equals("off", settings.TwoStep)
}

func (test *SessionSettingsServiceTest) scanning() *SessionSettingsService {
	settings := test.settings()
	settings.StartTwoStep()
	settings.SetUpTwoStepWith("right", "")
	return settings
}

func (test *SessionSettingsServiceTest) TestRetryTwoStepReadsTheStatusAgain() {
	settings := test.settings()
	settings.RetryTwoStep()
	Assert.Equals("on", settings.TwoStep)
}

func (test *SessionSettingsServiceTest) TestStartTwoStepResetAsksForTheCodeToo() {
	settings := test.settings()
	settings.StartTwoStepReset()
	Assert.Equals("twoStepPassword", settings.Step)
	Assert.IsTrue(settings.Resetting)
}

func (test *SessionSettingsServiceTest) TestShowCodeMovesToTheLastStep() {
	settings := test.scanning()
	settings.ShowCode()
	Assert.Equals("twoStepCode", settings.Step)
}

func (test *SessionSettingsServiceTest) TestBackFromTheCodeShowsTheScan() {
	settings := test.scanning()
	settings.ShowCode()
	settings.Back()
	Assert.Equals("twoStepScan", settings.Step)
}

func (test *SessionSettingsServiceTest) TestBackFromTheScanAsksForThePassword() {
	settings := test.scanning()
	settings.Back()
	Assert.Equals("twoStepPassword", settings.Step)
}

func (test *SessionSettingsServiceTest) TestCopyKeyConfirmsTheCopy() {
	settings := test.scanning()
	BrowserFakes{}.Answer("clipboard", "writeText", true)
	settings.CopyKey()
	Assert.Equals("keyCopied", settings.Notice)
}

func (test *SessionSettingsServiceTest) TestDismissNoticeHidesIt() {
	settings := test.scanning()
	BrowserFakes{}.Answer("clipboard", "writeText", true)
	settings.CopyKey()
	settings.DismissNotice()
	Assert.Equals("", settings.Notice)
}

func (test *SessionSettingsServiceTest) TestSecretGroupsSplitsTheKeyByFour() {
	settings := test.scanning()
	Assert.Equals("JBSW Y3DP EHPK 3PXP", settings.SecretGroups())
}

func (test *SessionSettingsServiceTest) TestShowPasswordsWithKeepsWhatWasTyped() {
	settings := test.settings()
	settings.StartPasswordChange()
	settings.ShowPasswordsWith(true, "right", "longer-and-better")
	Assert.IsTrue(settings.ShowPasswords)
	Assert.Equals("right", settings.ShownCurrent)
	Assert.Equals("longer-and-better", settings.ShownNew)
}
