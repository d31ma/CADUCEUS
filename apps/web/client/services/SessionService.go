// @Service
type SessionService struct {
	Email           string
	Password        string
	OneTimeCode     string
	Remember        bool
	Step            string
	Problem         string
	Adding          bool
	SignedOutDomain string
	// @LocalState
	RecentDomains []SessionRecentDomain
	// @Needs
	realm *SessionRealmService
	// @Needs
	tokens *SessionTokenService
	// @Needs
	accounts *SessionAccountService
	// @Needs
	problems *SessionProblemService
}

// @OnStart
func (session *SessionService) Start() {
	session.Remember = true
	session.accounts.Restore(session.tokens.Restore())
	session.Step = session.stepForAccounts()
}

func (session *SessionService) SignIn() {
	domainId := session.realm.DomainOf(session.Email)
	session.Problem = session.invalidWhenEmpty(domainId)
	session.signInWhenClear(domainId)
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
	session.Step = session.stepAfterSignOut()
}

func (session *SessionService) SignOutAll() {
	session.SignedOutDomain = ""
	session.signOutEach(session.accounts.Accounts)
	session.Step = session.stepAfterSignOut()
}

// @OnEvent
func (session *SessionService) Shown(changed VisibilityChanged) {
	failure := session.checkWhenShown(changed.Visible)
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
	kept := []SessionRecentDomain{}
	for _, recent := range session.RecentDomains {
		kept = session.keepRecent(kept, recent, domainId)
	}
	return kept
}

func (session *SessionService) keepRecent(kept []SessionRecentDomain, recent SessionRecentDomain, domainId string) []SessionRecentDomain {
	if recent.DomainId == domainId {
		return kept
	}
	return append(kept, recent)
}

func (session *SessionService) stepSignIn() string {
	return "signIn"
}

func (session *SessionService) stepSignedIn() string {
	return "signedIn"
}
