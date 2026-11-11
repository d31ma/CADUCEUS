import "strings"

// @Service
type SessionSettingsService struct {
	TwoStep         string
	Step            string
	Problem         string
	Notice          string
	CurrentPassword string
	NewPassword     string
	OneTimeCode     string
	Code            string
	ShowPasswords   bool
	ShownCurrent    string
	ShownNew        string
	Resetting       bool
	TwoStepSecret   string
	TwoStepQrImage  string
	setupKey        string
	// @Needs
	session *SessionService
	// @Needs
	identity *IdentityClient
	// @Needs
	problems *SessionProblemService
	// @Needs
	clipboard Clipboard
}

func (settings *SessionSettingsService) RetryTwoStep() {
	status, failure := settings.readTwoStep()
	status, failure = settings.retryAfterRenew(status, failure)
	settings.TwoStep = settings.twoStepState(status.Enabled, failure)
}

func (settings *SessionSettingsService) StartPasswordChange() {
	settings.begin(settings.stepPasswordChange())
}

func (settings *SessionSettingsService) ChangePasswordWith(currentPassword string, newPassword string, oneTimeCode string) {
	settings.CurrentPassword = currentPassword
	settings.NewPassword = newPassword
	settings.OneTimeCode = oneTimeCode
	settings.changePassword()
}

func (settings *SessionSettingsService) ShowPasswordsWith(show bool, currentPassword string, newPassword string) {
	settings.ShowPasswords = show
	settings.ShownCurrent = currentPassword
	settings.ShownNew = newPassword
}

func (settings *SessionSettingsService) changePassword() {
	change := SessionPasswordChange{CurrentPassword: settings.CurrentPassword, NewPassword: settings.NewPassword, OneTimeCode: settings.OneTimeCode}
	failure := settings.session.ChangePassword(change)
	settings.finish(settings.problems.OfAccountChange(failure, "passwordPolicy"), "passwordChanged")
}

func (settings *SessionSettingsService) StartTwoStep() {
	settings.begin(settings.stepPassword())
	settings.Resetting = false
}

func (settings *SessionSettingsService) StartTwoStepReset() {
	settings.begin(settings.stepPassword())
	settings.Resetting = true
}

func (settings *SessionSettingsService) SetUpTwoStepWith(currentPassword string, oneTimeCode string) {
	settings.CurrentPassword = currentPassword
	settings.OneTimeCode = oneTimeCode
	settings.setUpTwoStep()
}

func (settings *SessionSettingsService) setUpTwoStep() {
	request := SessionTwoStepSetupRequest{CurrentPassword: settings.CurrentPassword, OneTimeCode: settings.OneTimeCode}
	setup, failure := BehaviorsAnswer[SessionTwoStepSetup](settings.identity.SetUpTwoStep(settings.bearer(), request))
	settings.Problem = settings.problems.OfAccountChange(failure, settings.wrongPassword())
	settings.CurrentPassword = ""
	settings.OneTimeCode = ""
	settings.showSetupWhenClear(setup)
}

func (settings *SessionSettingsService) ShowCode() {
	settings.Problem = ""
	settings.Step = "twoStepCode"
}

func (settings *SessionSettingsService) Back() {
	settings.Problem = ""
	settings.Step = settings.previousStep()
}

func (settings *SessionSettingsService) CopyKey() {
	failure := settings.clipboard.WriteText(settings.TwoStepSecret)
	settings.Notice = settings.noticeWhenCopied(failure)
}

func (settings *SessionSettingsService) ConfirmTwoStepWith(code string) {
	settings.Code = code
	settings.confirmTwoStep()
}

func (settings *SessionSettingsService) confirmTwoStep() {
	_, failure := settings.identity.ConfirmTwoStep(settings.bearer(), SessionTwoStepConfirm{SetupKey: settings.setupKey, Code: settings.Code})
	settings.Code = ""
	settings.finishSetup(settings.problems.OfAccountChange(failure, "invalidCode"))
}

func (settings *SessionSettingsService) StartTwoStepRemoval() {
	settings.begin("twoStepRemove")
}

func (settings *SessionSettingsService) RemoveTwoStepWith(currentPassword string, oneTimeCode string) {
	settings.CurrentPassword = currentPassword
	settings.OneTimeCode = oneTimeCode
	settings.removeTwoStep()
}

func (settings *SessionSettingsService) removeTwoStep() {
	request := SessionTwoStepRemove{CurrentPassword: settings.CurrentPassword, OneTimeCode: settings.OneTimeCode}
	_, failure := settings.identity.RemoveTwoStep(settings.bearer(), request)
	problem := settings.problems.OfAccountChange(failure, settings.wrongPassword())
	settings.TwoStep = settings.stateAfter(problem, settings.off())
	settings.finish(problem, "twoStepOff")
}

func (settings *SessionSettingsService) DismissNotice() {
	settings.Notice = ""
}

func (settings *SessionSettingsService) Cancel() {
	settings.begin("")
}

func (settings *SessionSettingsService) SecretGroups() string {
	return strings.Join(settings.groupsOf(settings.TwoStepSecret), " ")
}

func (settings *SessionSettingsService) groupsOf(secret string) []string {
	groups := []string{}
	for start := 0; start < len(secret); start += settings.groupSize() {
		groups = append(groups, secret[start:min(start+settings.groupSize(), len(secret))])
	}
	return groups
}

func (settings *SessionSettingsService) begin(step string) {
	settings.clear()
	settings.Notice = ""
	settings.Step = step
}

func (settings *SessionSettingsService) finish(problem string, notice string) {
	settings.CurrentPassword = ""
	settings.NewPassword = ""
	settings.OneTimeCode = ""
	settings.Problem = problem
	settings.endWhenClear(notice)
}

func (settings *SessionSettingsService) finishSetup(problem string) {
	settings.Problem = problem
	settings.TwoStep = settings.stateAfter(problem, settings.on())
	settings.endWhenClear("twoStepOn")
}

func (settings *SessionSettingsService) endWhenClear(notice string) {
	if settings.Problem != "" {
		return
	}
	settings.clear()
	settings.Notice = notice
	settings.Step = ""
}

func (settings *SessionSettingsService) showSetupWhenClear(setup SessionTwoStepSetup) {
	if settings.Problem != "" {
		return
	}
	settings.TwoStepSecret = setup.Secret
	settings.TwoStepQrImage = setup.QrImage
	settings.setupKey = setup.SetupKey
	settings.Step = "twoStepScan"
}

func (settings *SessionSettingsService) previousStep() string {
	steps := map[string]string{"twoStepCode": "twoStepScan", "twoStepScan": settings.stepPassword()}
	return steps[settings.Step]
}

func (settings *SessionSettingsService) noticeWhenCopied(failure error) string {
	if failure != nil {
		return ""
	}
	return "keyCopied"
}

func (settings *SessionSettingsService) clear() {
	settings.CurrentPassword = ""
	settings.NewPassword = ""
	settings.OneTimeCode = ""
	settings.Code = ""
	settings.Problem = ""
	settings.ShowPasswords = false
	settings.ShownCurrent = ""
	settings.ShownNew = ""
	settings.TwoStepSecret = ""
	settings.TwoStepQrImage = ""
	settings.setupKey = ""
}

func (settings *SessionSettingsService) bearer() string {
	accessToken, _ := settings.session.AccessToken()
	return "Bearer " + accessToken
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

func (settings *SessionSettingsService) stateAfter(problem string, state string) string {
	return map[bool]string{true: state, false: settings.TwoStep}[problem == ""]
}

func (settings *SessionSettingsService) stepPasswordChange() string {
	return "password"
}

func (settings *SessionSettingsService) stepPassword() string {
	return "twoStepPassword"
}

func (settings *SessionSettingsService) wrongPassword() string {
	return "wrongPassword"
}

func (settings *SessionSettingsService) on() string {
	return "on"
}

func (settings *SessionSettingsService) off() string {
	return "off"
}

func (settings *SessionSettingsService) unauthorized() int {
	return 401
}

func (settings *SessionSettingsService) groupSize() int {
	return 4
}
