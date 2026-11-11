import (
	"slices"
	"strings"
)

// @Service
type AdminSessionService struct {
	Step        string
	Problem     string
	Busy        bool
	Address     string
	DomainId    string
	Initials    string
	DisplayName string
	Operator    bool
	DomainAdmin bool
	// @SessionState
	SessionKey                string
	accessToken               string
	expiresAtMs               float64
	applicationEventPublisher ApplicationEventPublisher
	// @Needs
	identityClient *IdentityClient
	// @Needs
	problems *AdminProblemService
	// @Needs
	clock Clock
}

// @OnStart
func (session *AdminSessionService) Start() {
	session.Step = session.signedOut()
	session.applicationEventPublisher.PublishEvent(AdminPageStarted{Started: true}, false)
}

// @OnEvent
// @InBackground
func (session *AdminSessionService) Resume(started AdminPageStarted) {
	session.renewWhenSaved(session.SessionKey)
}

func (session *AdminSessionService) SignInWith(address string, password string, oneTimeCode string) {
	session.Busy = true
	session.Address = address
	answer, failure := session.identityClient.SignIn(SignInRequest{Address: address, Password: password, OneTimeCode: oneTimeCode})
	session.Busy = false
	session.keep(answer)
	session.Problem = session.signInProblem(session.problems.Of(failure))
}

func (session *AdminSessionService) SignOut() {
	session.identityClient.SignOut(RefreshRequest{SessionKey: session.SessionKey})
	session.keep(Session{})
	session.Problem = ""
}

func (session *AdminSessionService) Authorization() string {
	session.renewWhenDue(session.now())
	return "Bearer " + session.accessToken
}

func (session *AdminSessionService) renewWhenSaved(sessionKey string) {
	if sessionKey == "" {
		return
	}
	session.renew()
}

func (session *AdminSessionService) renewWhenDue(now float64) {
	if now < session.expiresAtMs-session.renewMarginMs() {
		return
	}
	session.renew()
}

func (session *AdminSessionService) renew() {
	answer, failure := session.identityClient.Refresh(RefreshRequest{SessionKey: session.SessionKey})
	session.keep(answer)
	session.Problem = session.expiredProblem(failure)
}

func (session *AdminSessionService) keep(answer Session) {
	session.accessToken = answer.AccessToken
	session.expiresAtMs = session.now() + answer.ExpiresIn*session.millisecondsPerSecond()
	session.SessionKey = answer.SessionKey
	session.remember(answer.Me)
	session.Step = session.accessStep(session.isAdmin())
	session.Step = session.signedOutWhen(session.accessToken == "")
	session.announce(session.Step)
}

func (session *AdminSessionService) remember(me Me) {
	session.DomainId = me.DomainId
	session.Initials = session.initials(me.DomainId)
	session.DisplayName = me.DisplayName
	session.Operator = session.hasRole(me.Roles, []string{"Operator", "mesail-operator"})
	session.DomainAdmin = session.hasRole(me.Roles, []string{"DomainAdmin", "mesail-admin"})
}

func (session *AdminSessionService) initials(domainId string) string {
	if len(domainId) < session.initialsLength() {
		return strings.ToUpper(domainId)
	}
	return strings.ToUpper(domainId[:1]) + domainId[1:session.initialsLength()]
}

func (session *AdminSessionService) hasRole(roles []string, wanted []string) bool {
	return slices.ContainsFunc(roles, func(role string) bool { return slices.Contains(wanted, role) })
}

func (session *AdminSessionService) isAdmin() bool {
	return session.Operator || session.DomainAdmin
}

func (session *AdminSessionService) accessStep(admin bool) string {
	if !admin {
		return "noRights"
	}
	return session.ready()
}

func (session *AdminSessionService) signedOutWhen(missing bool) string {
	if missing {
		return session.signedOut()
	}
	return session.Step
}

func (session *AdminSessionService) signInProblem(problem string) string {
	named, known := session.signInProblems()[problem]
	if !known {
		return problem
	}
	return named
}

func (session *AdminSessionService) signInProblems() map[string]string {
	return map[string]string{session.signedOut(): session.refused(), "forbidden": session.refused()}
}

func (session *AdminSessionService) expiredProblem(failure error) string {
	if failure == nil {
		return ""
	}
	return "expired"
}

func (session *AdminSessionService) announce(step string) {
	if step != session.ready() {
		return
	}
	signedIn := AdminSignedIn{DomainId: session.DomainId, Operator: session.Operator, DomainAdmin: session.DomainAdmin}
	session.applicationEventPublisher.PublishEvent(signedIn, false)
}

func (session *AdminSessionService) now() float64 {
	now, _ := session.clock.NowMs()
	return now
}

func (session *AdminSessionService) ready() string {
	return "ready"
}

func (session *AdminSessionService) signedOut() string {
	return "signedOut"
}

func (session *AdminSessionService) renewMarginMs() float64 {
	return 15000
}

func (session *AdminSessionService) millisecondsPerSecond() float64 {
	return 1000
}

func (session *AdminSessionService) refused() string {
	return "refused"
}

func (session *AdminSessionService) initialsLength() int {
	return 2
}
