import (
	"slices"
	"strings"
)

// @Service
type AdminSessionService struct {
	Step          string
	Problem       string
	Busy          bool
	Login         string
	Hint          string
	Username      string
	DomainId      string
	Initials      string
	DisplayName   string
	Operator      bool
	DomainAdmin   bool
	Domains       []DomainChoice
	Tiles         []DomainTile
	Switching     string
	SwitchProblem string
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
	logins *AdminLoginService
	// @Needs
	clock Clock
}

// @OnStart
func (session *AdminSessionService) Start() {
	session.Step = session.signedOut()
	session.Hint = session.logins.Hint("")
	session.applicationEventPublisher.PublishEvent(AdminPageStarted{Started: true}, false)
}

// @OnEvent
// @InBackground
func (session *AdminSessionService) Resume(started AdminPageStarted) {
	session.renewWhenSaved(session.SessionKey)
}

func (session *AdminSessionService) TypeLogin(login string) {
	session.Login = login
	session.Hint = session.logins.Hint(login)
}

func (session *AdminSessionService) SignInWith(login string, password string, oneTimeCode string) {
	session.TypeLogin(login)
	session.Problem = session.missingProblem(session.logins.Missing(login))
	session.sendWhenReady(session.Problem, SignInRequest{Login: session.logins.Cleaned(login), Password: password, OneTimeCode: oneTimeCode})
}

func (session *AdminSessionService) UseUsernameOnly() {
	session.TypeLogin(session.logins.UsernameOf(session.Login))
	session.Problem = "retype"
}

func (session *AdminSessionService) ChangeAddress() {
	session.Problem = ""
}

func (session *AdminSessionService) SwitchTo(domainId string) {
	session.Switching = domainId
	answer, failure := session.identityClient.SwitchDomain(DomainSwitch{SessionKey: session.SessionKey, DomainId: domainId})
	session.Switching = ""
	session.SwitchProblem = session.switchProblem(session.problems.Of(failure))
	session.keepWhenSwitched(answer, failure)
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

func (session *AdminSessionService) sendWhenReady(problem string, request SignInRequest) {
	if problem != "" {
		return
	}
	session.send(request)
}

func (session *AdminSessionService) send(request SignInRequest) {
	session.Busy = true
	answer, failure := session.identityClient.SignIn(request)
	session.Busy = false
	session.keep(answer)
	session.Problem = session.signInProblem(session.problems.Of(failure))
}

func (session *AdminSessionService) keepWhenSwitched(answer Session, failure error) {
	if failure != nil {
		return
	}
	session.keep(answer)
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
	session.Domains = answer.Domains
	session.remember(answer.Me)
	session.Tiles = session.tilesOf(answer.Domains)
	session.Step = session.accessStep(session.isAdmin())
	session.Step = session.signedOutWhen(session.accessToken == "")
	target := session.moveTarget(session.Step)
	session.announceWhen(session.Step == session.ready() && target == "")
	session.switchWhen(target)
}

func (session *AdminSessionService) remember(me Me) {
	session.DomainId = me.DomainId
	session.Username = me.Username
	session.Initials = session.initials(me.DomainId)
	session.DisplayName = me.DisplayName
	session.Operator = session.hasRole(me.Roles, []string{"Operator", "mesail-operator"})
	session.DomainAdmin = session.hasRole(me.Roles, []string{"DomainAdmin", "mesail-admin"})
}

func (session *AdminSessionService) tilesOf(domains []DomainChoice) []DomainTile {
	tiles := []DomainTile{}
	for _, domain := range slices.DeleteFunc(slices.Clone(domains), session.notAdministered) {
		tiles = append(tiles, DomainTile{DomainId: domain.DomainId, Initials: session.initials(domain.DomainId), Current: domain.DomainId == session.DomainId})
	}
	return tiles
}

func (session *AdminSessionService) notAdministered(domain DomainChoice) bool {
	return !domain.Admin
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
	return session.Operator || len(session.Tiles) > 0
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

func (session *AdminSessionService) moveTarget(step string) string {
	if step != session.ready() || session.Operator || session.DomainAdmin {
		return ""
	}
	return session.firstTile()
}

func (session *AdminSessionService) firstTile() string {
	return append(slices.Clone(session.Tiles), DomainTile{})[0].DomainId
}

func (session *AdminSessionService) announceWhen(ready bool) {
	if !ready {
		return
	}
	signedIn := AdminSignedIn{DomainId: session.DomainId, Operator: session.Operator, DomainAdmin: session.DomainAdmin}
	session.applicationEventPublisher.PublishEvent(signedIn, false)
}

func (session *AdminSessionService) switchWhen(target string) {
	if target == "" {
		return
	}
	session.SwitchTo(target)
}

func (session *AdminSessionService) missingProblem(missing bool) string {
	if missing {
		return "loginMissing"
	}
	return ""
}

func (session *AdminSessionService) signInProblem(problem string) string {
	named, known := session.signInProblems(session.logins.Forbidden(session.Login))[problem]
	if !known {
		return problem
	}
	return named
}

func (session *AdminSessionService) signInProblems(forbidden string) map[string]string {
	return map[string]string{session.signedOut(): session.refused(), "invalid": session.refused(), "forbidden": forbidden}
}

func (session *AdminSessionService) switchProblem(problem string) string {
	if problem == "notFound" {
		return "notMember"
	}
	return problem
}

func (session *AdminSessionService) expiredProblem(failure error) string {
	if failure == nil {
		return ""
	}
	return "expired"
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
