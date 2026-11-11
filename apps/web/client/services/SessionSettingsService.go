// @Service
type SessionSettingsService struct {
	TwoStep         string
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

// @OnEvent
// @InBackground
func (settings *SessionSettingsService) LoadTwoStep(restored SessionRestored) {
	status, failure := settings.readTwoStep()
	status, failure = settings.retryAfterRenew(status, failure)
	settings.TwoStep = settings.twoStepState(status.Enabled, failure)
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
	problem := settings.problems.OfAccountChange(failure, settings.wrongPassword())
	settings.TwoStep = settings.stateAfter(problem, settings.off())
	settings.finish(problem, "twoStepOff")
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
	settings.TwoStep = settings.stateAfter(problem, settings.on())
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

func (settings *SessionSettingsService) readTwoStep() (SessionTwoStepStatus, error) {
	return BehaviorsAnswer[SessionTwoStepStatus](settings.identity.TwoStepStatus(settings.bearer()))
}

func (settings *SessionSettingsService) retryAfterRenew(status SessionTwoStepStatus, failure error) (SessionTwoStepStatus, error) {
	if BehaviorErrorStatus(failure) != settings.unauthorized() {
		return status, failure
	}
	settings.session.Renew()
	return settings.readTwoStep()
}

func (settings *SessionSettingsService) twoStepState(enabled bool, failure error) string {
	if failure != nil {
		return "unknown"
	}
	return map[bool]string{true: settings.on(), false: settings.off()}[enabled]
}

func (settings *SessionSettingsService) unauthorized() int {
	return 401
}

func (settings *SessionSettingsService) stateAfter(problem string, state string) string {
	return settings.pick(problem == "", state, settings.TwoStep)
}

func (settings *SessionSettingsService) pick(chosen bool, first string, otherwise string) string {
	if chosen {
		return first
	}
	return otherwise
}

func (settings *SessionSettingsService) on() string {
	return "on"
}

func (settings *SessionSettingsService) off() string {
	return "off"
}
