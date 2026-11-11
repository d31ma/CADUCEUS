import "slices"

// @Service
type SessionService struct {
	Email           string
	Password        string
	OneTimeCode     string
	Remember        bool
	ShowPassword    bool
	Step            string
	Problem         string
	Adding          bool
	SignedOutDomain string
	// @LocalState
	RecentDomains []SessionRecentDomain
	// @SessionState
	SessionKeys map[string]string
	// @Needs
	realm *SessionRealmService
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
	session.applicationEventPublisher.PublishEvent(SessionRestoreRequested{}, false)
}

// @OnEvent
// @InBackground
func (session *SessionService) Restore(requested SessionRestoreRequested) {
	session.accounts.Restore(session.tokens.Restore(session.SessionKeys))
	session.SessionKeys = session.tokens.SavedKeys()
	session.Step = session.stepForAccounts()
	session.applicationEventPublisher.PublishEvent(SessionRestored{SignedIn: session.Step == session.stepSignedIn()}, true)
}

func (session *SessionService) SignIn() {
	domainId := session.realm.DomainOf(session.Email)
	session.Problem = session.invalidWhenEmpty(domainId)
	session.signInWhenClear(domainId)
	session.SessionKeys = session.tokens.SavedKeys()
}

func (session *SessionService) TogglePassword() {
	session.ShowPassword = !session.ShowPassword
}

func (session *SessionService) UseRecent(recent SessionRecentDomain) {
	session.Email = recent.Email
	session.Problem = ""
}

func (session *SessionService) Forget(domainId string) {
	session.RecentDomains = session.recentWithout(domainId)
}

func (session *SessionService) ForgetAll() {
	session.RecentDomains = []SessionRecentDomain{}
}

func (session *SessionService) AddDomain() {
	session.Adding = true
	session.clearForm()
	session.Step = session.stepSignIn()
}

func (session *SessionService) CancelAdd() {
	session.Adding = false
	session.clearForm()
	session.Step = session.stepForAccounts()
}

func (session *SessionService) SignInAgain() {
	session.Email = session.accounts.Current().Address
	session.Adding = false
	session.Step = session.stepSignIn()
}

func (session *SessionService) Switch(domainId string) {
	session.accounts.Switch(domainId)
}

func (session *SessionService) SignOut() {
	session.SignedOutDomain = session.accounts.CurrentDomain
	session.signOutOf(session.accounts.CurrentDomain)
	session.SessionKeys = session.tokens.SavedKeys()
	session.Step = session.stepAfterSignOut()
}

func (session *SessionService) SignOutAll() {
	session.SignedOutDomain = ""
	session.signOutEach(session.accounts.Accounts)
	session.SessionKeys = session.tokens.SavedKeys()
	session.Step = session.stepAfterSignOut()
}

func (session *SessionService) AccessToken() (string, error) {
	accessToken, failure := session.tokens.AccessToken(session.accounts.CurrentDomain)
	session.SessionKeys = session.tokens.SavedKeys()
	return accessToken, failure
}

func (session *SessionService) Renew() (string, error) {
	failure := session.tokens.Renew(session.accounts.CurrentDomain)
	session.SessionKeys = session.tokens.SavedKeys()
	session.Step = session.expiredWhenFailed(failure)
	return session.AccessToken()
}

func (session *SessionService) ChangePassword(change SessionPasswordChange) error {
	failure := session.tokens.ChangePassword(session.accounts.CurrentDomain, change)
	session.SessionKeys = session.tokens.SavedKeys()
	return failure
}

// @OnEvent
// @InBackground
func (session *SessionService) Shown(changed VisibilityChanged) {
	failure := session.checkWhenShown(changed.Visible)
	session.SessionKeys = session.tokens.SavedKeys()
	session.Step = session.expiredWhenFailed(failure)
}

func (session *SessionService) signInWhenClear(domainId string) {
	if session.Problem != "" {
		return
	}
	session.signInTo(domainId)
}

func (session *SessionService) signInTo(domainId string) {
	request := SessionSignInRequest{Address: session.Email, Password: session.Password, OneTimeCode: session.OneTimeCode}
	me, failure := session.tokens.SignIn(request)
	session.Problem = session.problems.OfFailure(failure)
	session.Password = ""
	session.OneTimeCode = ""
	session.ShowPassword = false
	session.rememberWhenAsked(domainId)
	session.addWhenClear(me)
}

func (session *SessionService) addWhenClear(me Me) {
	if session.Problem != "" {
		return
	}
	session.accounts.Add(me)
	session.Adding = false
	session.Step = session.stepSignedIn()
}

func (session *SessionService) signOutEach(accounts []SessionAccount) {
	for _, account := range accounts {
		session.signOutOf(account.DomainId)
	}
}

func (session *SessionService) signOutOf(domainId string) {
	session.tokens.SignOut(domainId)
	session.accounts.Remove(domainId)
}

func (session *SessionService) checkWhenShown(visible bool) error {
	if !visible || session.Step != session.stepSignedIn() {
		return nil
	}
	_, failure := session.tokens.AccessToken(session.accounts.CurrentDomain)
	return failure
}

func (session *SessionService) clearForm() {
	session.Email = ""
	session.Password = ""
	session.OneTimeCode = ""
	session.Problem = ""
}

func (session *SessionService) invalidWhenEmpty(domainId string) string {
	if domainId == "" {
		return "invalidEmail"
	}
	return ""
}

func (session *SessionService) stepForAccounts() string {
	if len(session.accounts.Accounts) == 0 {
		return session.stepSignIn()
	}
	return session.stepSignedIn()
}

func (session *SessionService) stepAfterSignOut() string {
	if len(session.accounts.Accounts) == 0 {
		return "signedOut"
	}
	return session.stepSignedIn()
}

func (session *SessionService) expiredWhenFailed(failure error) string {
	if failure != nil {
		return "expired"
	}
	return session.Step
}

func (session *SessionService) rememberWhenAsked(domainId string) {
	if !session.Remember || session.Problem != "" {
		return
	}
	recent := SessionRecentDomain{DomainId: domainId, Email: session.Email}
	session.RecentDomains = append([]SessionRecentDomain{recent}, session.recentWithout(domainId)...)
}

func (session *SessionService) recentWithout(domainId string) []SessionRecentDomain {
	return slices.DeleteFunc(slices.Clone(session.RecentDomains), func(recent SessionRecentDomain) bool {
		return recent.DomainId == domainId
	})
}

func (session *SessionService) stepSignIn() string {
	return "signIn"
}

func (session *SessionService) stepSignedIn() string {
	return "signedIn"
}
