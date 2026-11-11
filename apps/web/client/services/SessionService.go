import "strings"

// @Service
type SessionService struct {
	Login         string
	ShownPassword string
	Password      string
	OneTimeCode   string
	Remember      bool
	ShowPassword  bool
	Busy          bool
	Step          string
	Problem       string
	ProblemDomain string
	Switching     string
	Notice        string
	// @LocalState
	RememberedUsername string
	// @SessionState
	SessionKey string
	// @Needs
	tokens *SessionTokenService
	// @Needs
	accounts *SessionAccountService
	// @Needs
	problems                  *SessionProblemService
	applicationEventPublisher ApplicationEventPublisher
}

// @OnStart
func (session *SessionService) Start() {
	session.Login = session.RememberedUsername
	session.Remember = session.RememberedUsername != ""
	session.applicationEventPublisher.PublishEvent(SessionRestoreRequested{}, false)
}

// @OnEvent
// @InBackground
func (session *SessionService) Restore(requested SessionRestoreRequested) {
	current, failure := session.tokens.Restore(session.SessionKey)
	session.SessionKey = session.tokens.SavedKey()
	session.accounts.Apply(current)
	session.Step = session.stepAfterRestore(current, failure)
	session.applicationEventPublisher.PublishEvent(SessionRestored{SignedIn: session.Step == session.stepSignedIn()}, true)
	session.announceDomain()
}

func (session *SessionService) SignInWith(login string, password string, oneTimeCode string, remember bool) {
	session.Login = strings.TrimSpace(login)
	session.Password = password
	session.OneTimeCode = oneTimeCode
	session.Remember = remember
	session.Problem = session.problems.OfLogin(session.Login)
	session.startSignInWhenClear()
}

func (session *SessionService) SignInAsUsername() {
	session.Login = session.UsernameOf(session.Login)
	session.OneTimeCode = ""
	session.Problem = ""
	session.startSignInWhenClear()
}

func (session *SessionService) ChangeAddress() {
	session.Problem = ""
	session.ProblemDomain = ""
	session.ShownPassword = session.Password
	session.Password = ""
}

func (session *SessionService) UseAnotherUsername() {
	session.RememberedUsername = ""
	session.Login = ""
	session.Remember = false
	session.Problem = ""
}

func (session *SessionService) ShowPasswordWith(show bool, login string, password string) {
	session.ShowPassword = show
	session.Login = login
	session.ShownPassword = password
}

func (session *SessionService) UsernameOf(login string) string {
	username, _, _ := strings.Cut(strings.ToLower(strings.TrimSpace(login)), session.at())
	return username
}

// @OnEvent
// @InBackground
func (session *SessionService) SignIn(requested SessionSignInRequested) {
	request := SessionSignInRequest{Login: session.Login, Password: session.Password, OneTimeCode: session.OneTimeCode}
	current, failure := session.tokens.SignIn(request)
	session.Problem = session.problems.OfSignIn(failure, session.Login)
	session.ProblemDomain = session.domainOf(strings.ToLower(session.Login))
	session.SessionKey = session.tokens.SavedKey()
	session.finishSignIn(current)
	session.Busy = false
	session.applicationEventPublisher.PublishEvent(SessionChanged{}, true)
}

func (session *SessionService) BackToSignIn() {
	session.Problem = ""
	session.Step = session.stepSignIn()
}

func (session *SessionService) Switch(domainId string) {
	session.Notice = ""
	session.requestUnlessCurrent(domainId)
}

// @OnEvent
// @InBackground
func (session *SessionService) SwitchTo(requested SessionSwitchRequested) {
	current, failure := session.tokens.Switch(requested.DomainId)
	session.SessionKey = session.tokens.SavedKey()
	session.afterSwitch(session.problems.OfSwitch(failure), requested.DomainId, current)
	session.Switching = ""
	session.applicationEventPublisher.PublishEvent(SessionChanged{}, true)
}

func (session *SessionService) SignOut() {
	session.tokens.SignOut()
	session.SessionKey = ""
	session.accounts.Clear()
	session.Notice = ""
	session.Step = "signedOut"
	session.announceDomain()
}

func (session *SessionService) SignInAgain() {
	session.Login = session.RememberedUsername
	session.Password = ""
	session.ShownPassword = ""
	session.Problem = ""
	session.Step = session.stepSignIn()
}

func (session *SessionService) ForgetUsername() {
	session.RememberedUsername = ""
	session.Login = ""
	session.Remember = false
}

func (session *SessionService) DismissNotice() {
	session.Notice = ""
}

func (session *SessionService) PasswordChosen() {
	session.accounts.Apply(session.tokens.Current())
	session.SessionKey = session.tokens.SavedKey()
	session.Step = session.stepAfterSignIn(session.tokens.Current())
	session.announceDomain()
}

func (session *SessionService) AccessToken() (string, error) {
	accessToken, failure := session.tokens.AccessToken()
	session.followTokens()
	return accessToken, failure
}

func (session *SessionService) Renew() (string, error) {
	failure := session.tokens.Renew()
	session.followTokens()
	session.Step = session.expiredWhenFailed(failure)
	return session.AccessToken()
}

func (session *SessionService) ChangePassword(change SessionPasswordChange) error {
	_, failure := session.tokens.ChangePassword(change)
	session.SessionKey = session.tokens.SavedKey()
	return failure
}

// @OnEvent
// @InBackground
func (session *SessionService) Shown(changed VisibilityChanged) {
	failure := session.checkWhenShown(changed.Visible)
	session.followTokens()
	session.Step = session.expiredWhenFailed(failure)
}

func (session *SessionService) requestUnlessCurrent(domainId string) {
	if domainId == session.accounts.CurrentDomain {
		return
	}
	session.Switching = domainId
	session.applicationEventPublisher.PublishEvent(SessionSwitchRequested{DomainId: domainId}, false)
}

func (session *SessionService) startSignInWhenClear() {
	if session.Problem != "" {
		return
	}
	session.Busy = true
	session.applicationEventPublisher.PublishEvent(SessionSignInRequested{}, false)
}

func (session *SessionService) finishSignIn(current Session) {
	session.OneTimeCode = ""
	session.ShowPassword = false
	session.ShownPassword = ""
	session.Password = session.passwordKeptFor(session.Problem)
	session.Step = session.stepAfterProblem(session.Problem)
	session.openWhenClear(current, session.rememberedUsername())
}

func (session *SessionService) openWhenClear(current Session, remembered string) {
	if session.Problem != "" {
		return
	}
	session.RememberedUsername = remembered
	session.accounts.Apply(current)
	session.Step = session.stepAfterSignIn(current)
	session.announceDomain()
}

func (session *SessionService) afterSwitch(problem string, domainId string, current Session) {
	switch problem {
	case "":
		session.switched(current)
	case "notMember":
		session.removed(domainId)
	case "ended":
		session.ended()
	default:
		session.Notice = "We could not open " + domainId + session.period() + " Try again in a minute."
	}
}

func (session *SessionService) switched(current Session) {
	session.accounts.Apply(current)
	session.Notice = "Now in " + current.Me.DomainId + session.period()
	session.announceDomain()
}

func (session *SessionService) removed(domainId string) {
	session.accounts.Remove(domainId)
	session.Notice = "You are no longer a member of " + domainId + ", so it is gone from your list."
}

func (session *SessionService) ended() {
	session.Step = "expired"
}

func (session *SessionService) followTokens() {
	current := session.tokens.Current()
	before := session.accounts.CurrentDomain
	session.SessionKey = session.tokens.SavedKey()
	session.followWhenSignedIn(current)
	session.noticeWhenMoved(before, session.accounts.CurrentDomain)
}

func (session *SessionService) followWhenSignedIn(current Session) {
	if current.SessionKey == "" {
		return
	}
	session.accounts.Apply(current)
}

func (session *SessionService) noticeWhenMoved(before string, after string) {
	quiet := map[string]bool{after: true, "": true}[before]
	if quiet {
		return
	}
	session.Notice = "You no longer have access to " + before + session.period() + " Now in " + after + session.period()
	session.announceDomain()
}

func (session *SessionService) announceDomain() {
	session.applicationEventPublisher.PublishEvent(SessionDomainChanged{DomainId: session.accounts.CurrentDomain}, true)
}

func (session *SessionService) checkWhenShown(visible bool) error {
	if !visible || session.Step != session.stepSignedIn() {
		return nil
	}
	_, failure := session.tokens.AccessToken()
	return failure
}

func (session *SessionService) domainOf(login string) string {
	_, domainId, _ := strings.Cut(strings.TrimSpace(login), session.at())
	return domainId
}

func (session *SessionService) rememberedUsername() string {
	if !session.Remember {
		return ""
	}
	return session.UsernameOf(session.Login)
}

func (session *SessionService) passwordKeptFor(problem string) string {
	if problem != "notAMember" {
		return ""
	}
	return session.Password
}

func (session *SessionService) stepAfterProblem(problem string) string {
	if problem == session.stepNoDomain() {
		return session.stepNoDomain()
	}
	return session.stepSignIn()
}

func (session *SessionService) stepAfterRestore(current Session, failure error) string {
	if failure != nil {
		return session.stepSignIn()
	}
	return session.stepAfterSignIn(current)
}

func (session *SessionService) stepAfterSignIn(current Session) string {
	return session.passwordStepOr(current.MustChangePassword, session.domainStep(current.Me.DomainId))
}

func (session *SessionService) passwordStepOr(mustChange bool, step string) string {
	if mustChange {
		return "choosePassword"
	}
	return step
}

func (session *SessionService) domainStep(domainId string) string {
	if domainId == session.installWide() {
		return "noDomainOperator"
	}
	return session.stepSignedIn()
}

func (session *SessionService) expiredWhenFailed(failure error) string {
	return session.expiredWhen(session.openSessionFailed(failure))
}

func (session *SessionService) openSessionFailed(failure error) bool {
	return failure != nil && session.Step == session.stepSignedIn()
}

func (session *SessionService) expiredWhen(ended bool) string {
	if ended {
		return "expired"
	}
	return session.Step
}

func (session *SessionService) period() string {
	return "."
}

func (session *SessionService) at() string {
	return "@"
}

func (session *SessionService) stepNoDomain() string {
	return "noDomain"
}

func (session *SessionService) installWide() string {
	return "*"
}

func (session *SessionService) stepSignIn() string {
	return "signIn"
}

func (session *SessionService) stepSignedIn() string {
	return "signedIn"
}
