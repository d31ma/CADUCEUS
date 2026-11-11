// @Service
type SessionSettingsService struct {
	Step            string
	Problem         string
	Message         string
	CurrentPassword string
	NewPassword     string
	OneTimeCode     string
	Code            string
	TwoStepSecret   string
	TwoStepQrImage  string
	setupKey        string
	// @Needs
	session *SessionService
	// @Needs
	identity *IdentityClient
	// @Needs
	problems *SessionProblemService
}

func (settings *SessionSettingsService) StartPasswordChange() {
	settings.begin("password")
}

func (settings *SessionSettingsService) ChangePassword() {
	change := SessionPasswordChange{CurrentPassword: settings.CurrentPassword, NewPassword: settings.NewPassword, OneTimeCode: settings.OneTimeCode}
	failure := settings.session.ChangePassword(change)
	settings.finish(settings.problems.OfAccountChange(failure, "passwordPolicy"), "passwordChanged")
}

func (settings *SessionSettingsService) StartTwoStep() {
	settings.begin("twoStepPassword")
}

func (settings *SessionSettingsService) SetUpTwoStep() {
	request := SessionTwoStepSetupRequest{CurrentPassword: settings.CurrentPassword}
	setup, failure := BehaviorsAnswer[SessionTwoStepSetup](settings.identity.SetUpTwoStep(settings.bearer(), request))
	settings.Problem = settings.problems.OfAccountChange(failure, settings.wrongPassword())
	settings.CurrentPassword = ""
	settings.showSetupWhenClear(setup)
}

func (settings *SessionSettingsService) ConfirmTwoStep() {
	_, failure := settings.identity.ConfirmTwoStep(settings.bearer(), SessionTwoStepConfirm{SetupKey: settings.setupKey, Code: settings.Code})
	settings.Code = ""
	settings.finishSetup(settings.problems.OfAccountChange(failure, "invalidCode"))
}

func (settings *SessionSettingsService) StartTwoStepRemoval() {
	settings.begin("twoStepRemove")
}

func (settings *SessionSettingsService) RemoveTwoStep() {
	request := SessionTwoStepRemove{CurrentPassword: settings.CurrentPassword, OneTimeCode: settings.OneTimeCode}
	_, failure := settings.identity.RemoveTwoStep(settings.bearer(), request)
	settings.finish(settings.problems.OfAccountChange(failure, settings.wrongPassword()), "twoStepOff")
}

func (settings *SessionSettingsService) Cancel() {
	settings.begin("")
}

func (settings *SessionSettingsService) begin(step string) {
	settings.clear()
	settings.Message = ""
	settings.Step = step
}

func (settings *SessionSettingsService) finish(problem string, message string) {
	settings.CurrentPassword = ""
	settings.NewPassword = ""
	settings.OneTimeCode = ""
	settings.Problem = problem
	settings.endWhenClear(message)
}

func (settings *SessionSettingsService) finishSetup(problem string) {
	settings.Problem = problem
	settings.endWhenClear("twoStepOn")
}

func (settings *SessionSettingsService) endWhenClear(message string) {
	if settings.Problem != "" {
		return
	}
	settings.clear()
	settings.Message = message
	settings.Step = ""
}

func (settings *SessionSettingsService) showSetupWhenClear(setup SessionTwoStepSetup) {
	if settings.Problem != "" {
		return
	}
	settings.TwoStepSecret = setup.Secret
	settings.TwoStepQrImage = setup.QrImage
	settings.setupKey = setup.SetupKey
	settings.Step = "twoStepConfirm"
}

func (settings *SessionSettingsService) clear() {
	settings.CurrentPassword = ""
	settings.NewPassword = ""
	settings.OneTimeCode = ""
	settings.Code = ""
	settings.Problem = ""
	settings.TwoStepSecret = ""
	settings.TwoStepQrImage = ""
	settings.setupKey = ""
}

func (settings *SessionSettingsService) bearer() string {
	accessToken, _ := settings.session.AccessToken()
	return "Bearer " + accessToken
}

func (settings *SessionSettingsService) wrongPassword() string {
	return "wrongPassword"
}
