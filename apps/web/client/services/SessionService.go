// @Service
type SessionService struct {
	Email        string
	Step         string
	Problem      string
	DomainId     string
	SignInHost   string
	AuthorizeUrl string
	SignOutUrl   string
	Remember     bool
	// @LocalState
	RecentDomains []SessionRecentDomain
	// @SessionState
	Pending SessionPending
	// @Needs
	proof *SessionProofService
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
	session.Step = session.stepEmail()
	session.Problem = session.unavailableWhen(session.realm.Load())
	session.Step = session.stepFor(session.Problem, session.Step)
}

func (session *SessionService) Continue() {
	session.DomainId = session.realm.DomainOf(session.Email)
	session.Problem = session.invalidWhenEmpty(session.DomainId)
	session.Step = session.domainStepWhenClear(session.Problem)
	session.prepareWhenClear(session.Problem)
}

func (session *SessionService) ContinueAs(recent SessionRecentDomain) {
	session.Email = recent.Email
	session.Continue()
}

func (session *SessionService) Forget(domainId string) {
	session.RecentDomains = session.recentWithout(domainId)
}

func (session *SessionService) ForgetAll() {
	session.RecentDomains = []SessionRecentDomain{}
}

func (session *SessionService) UseDifferentEmail() {
	session.Problem = ""
	session.Step = session.stepEmail()
}

func (session *SessionService) Leave() {
	session.Step = "redirecting"
	session.rememberWhenAsked()
}

func (session *SessionService) Cancel() {
	session.Step = session.stepDomain()
}

func (session *SessionService) Finish(code string, state string) {
	problem := session.problems.OfState(session.Pending.State, state)
	problem = session.exchangeWhenClear(problem, code)
	me, problem := session.loadWhenClear(problem)
	session.Problem = problem
	session.Step = session.stepFor(problem, session.stepSignedIn())
	session.addWhenClear(me, problem)
}

func (session *SessionService) Fail(signInError string) {
	session.Problem = session.problems.OfSignInError(signInError)
	session.Step = session.stepProblem()
}

func (session *SessionService) Switch(domainId string) {
	session.accounts.Switch(domainId)
	session.SignOutUrl = session.realm.LogoutUrl(domainId, session.tokens.IdToken(domainId))
}

func (session *SessionService) SignOut() {
	session.tokens.Forget(session.accounts.CurrentDomain)
	session.Step = "signedOut"
}

// @OnEvent
func (session *SessionService) Shown(changed VisibilityChanged) {
	failure := session.checkWhenShown(changed.Visible)
	session.Step = session.expiredWhenFailed(failure)
}

func (session *SessionService) checkWhenShown(visible bool) error {
	if !visible || session.Step != session.stepSignedIn() {
		return nil
	}
	_, failure := session.tokens.AccessToken(session.accounts.CurrentDomain)
	return failure
}

func (session *SessionService) prepareWhenClear(problem string) {
	if problem != "" {
		return
	}
	session.prepare()
}

func (session *SessionService) prepare() {
	key, failure := session.proof.NewKey()
	session.Pending = SessionPending{DomainId: session.DomainId, Email: session.Email, State: key.State, Verifier: key.Verifier}
	session.AuthorizeUrl = session.realm.AuthorizeUrl(session.Pending, key.Challenge)
	session.SignInHost = session.realm.SignInHost(session.DomainId)
	session.Problem = session.unavailableWhen(failure)
	session.Step = session.stepFor(session.Problem, session.Step)
}

func (session *SessionService) exchangeWhenClear(problem string, code string) string {
	if problem != "" {
		return problem
	}
	return session.problems.OfFailure(session.tokens.Exchange(session.Pending, code, session.realm.RedirectUri()))
}

func (session *SessionService) loadWhenClear(problem string) (Me, string) {
	if problem != "" {
		return Me{}, problem
	}
	accessToken, _ := session.tokens.AccessToken(session.Pending.DomainId)
	me, failure := session.accounts.Load(accessToken)
	return me, session.problems.OfFailure(failure)
}

func (session *SessionService) addWhenClear(me Me, problem string) {
	if problem != "" {
		return
	}
	session.accounts.Add(me)
	session.Pending = SessionPending{}
	session.Switch(me.DomainId)
}

func (session *SessionService) invalidWhenEmpty(domainId string) string {
	if domainId == "" {
		return "invalidEmail"
	}
	return ""
}

func (session *SessionService) domainStepWhenClear(problem string) string {
	if problem != "" {
		return session.stepEmail()
	}
	return session.stepDomain()
}

func (session *SessionService) unavailableWhen(failure error) string {
	if failure != nil {
		return session.problems.Unavailable()
	}
	return ""
}

func (session *SessionService) stepFor(problem string, otherwise string) string {
	if problem != "" {
		return session.stepProblem()
	}
	return otherwise
}

func (session *SessionService) expiredWhenFailed(failure error) string {
	if failure != nil {
		return "expired"
	}
	return session.Step
}

func (session *SessionService) rememberWhenAsked() {
	if !session.Remember {
		return
	}
	recent := SessionRecentDomain{DomainId: session.DomainId, Email: session.Email}
	session.RecentDomains = append([]SessionRecentDomain{recent}, session.recentWithout(session.DomainId)...)
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

func (session *SessionService) stepEmail() string {
	return "email"
}

func (session *SessionService) stepDomain() string {
	return "domain"
}

func (session *SessionService) stepProblem() string {
	return "problem"
}

func (session *SessionService) stepSignedIn() string {
	return "signedIn"
}
