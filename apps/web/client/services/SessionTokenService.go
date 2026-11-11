import (
	"errors"
	"math"
)

// @Service
type SessionTokenService struct {
	key        string
	tokens     SessionTokens
	session    Session
	refreshing bool
	// @Needs
	identity *IdentityClient
	// @Needs
	clock Clock
}

func (service *SessionTokenService) SignIn(request SessionSignInRequest) (Session, error) {
	session, failure := BehaviorsAnswer[Session](service.identity.SignIn(request))
	return session, service.keepUnlessFailed(session, failure)
}

func (service *SessionTokenService) Restore(key string) (Session, error) {
	service.key = key
	failure := service.restoreWhenKept(key)
	return service.session, failure
}

func (service *SessionTokenService) SavedKey() string {
	return service.key
}

func (service *SessionTokenService) Current() Session {
	return service.session
}

func (service *SessionTokenService) AccessToken() (string, error) {
	failure := service.refreshWhenDue()
	return service.tokens.AccessToken, failure
}

func (service *SessionTokenService) Renew() error {
	return service.renewUnlessRefreshing()
}

func (service *SessionTokenService) Switch(domainId string) (Session, error) {
	return service.switchUnlessRefreshing(domainId)
}

func (service *SessionTokenService) ChangePassword(change SessionPasswordChange) (Session, error) {
	accessToken, failure := service.AccessToken()
	session, answerFailure := BehaviorsAnswer[Session](service.identity.ChangePassword("Bearer "+accessToken, change))
	joined := errors.Join(failure, answerFailure)
	return session, service.keepUnlessFailed(session, joined)
}

func (service *SessionTokenService) SignOut() error {
	_, failure := service.identity.SignOut(SessionRefreshRequest{SessionKey: service.key})
	service.forget()
	return failure
}

func (service *SessionTokenService) restoreWhenKept(key string) error {
	if key == "" {
		return errors.New("no session on this tab")
	}
	return service.refreshMarked()
}

func (service *SessionTokenService) refreshMarked() error {
	service.refreshing = true
	defer service.stopRefreshing()
	session, failure := BehaviorsAnswer[Session](service.identity.Refresh(SessionRefreshRequest{SessionKey: service.key}))
	session, failure = service.openDefaultWhenRemoved(session, failure, service.statusOf(failure))
	return service.keepOrForget(session, failure)
}

func (service *SessionTokenService) switchMarked(domainId string) (Session, error) {
	service.refreshing = true
	defer service.stopRefreshing()
	session, failure := service.answerSwitch(domainId)
	return session, service.keepOrForget(session, failure)
}

func (service *SessionTokenService) stopRefreshing() {
	service.refreshing = false
}

func (service *SessionTokenService) answerSwitch(domainId string) (Session, error) {
	request := SessionDomainSwitch{SessionKey: service.key, DomainId: domainId}
	return BehaviorsAnswer[Session](service.identity.SwitchDomain(request))
}

func (service *SessionTokenService) openDefaultWhenRemoved(session Session, failure error, status int) (Session, error) {
	if status != service.forbidden() {
		return session, failure
	}
	return service.answerSwitch(service.defaultDomain())
}

func (service *SessionTokenService) refreshWhenDue() error {
	now, failure := service.clock.NowMs()
	refreshFailure := service.refreshIfPassed(service.nowUnlessFailed(now, failure), service.refreshAtMs())
	return errors.Join(failure, refreshFailure)
}

func (service *SessionTokenService) nowUnlessFailed(now float64, failure error) float64 {
	if failure != nil {
		return math.Inf(-1)
	}
	return now
}

func (service *SessionTokenService) refreshAtMs() float64 {
	if service.refreshing {
		return math.Inf(1)
	}
	return service.tokens.ExpiresAtMs - service.refreshMarginMs()
}

func (service *SessionTokenService) refreshIfPassed(now float64, refreshAt float64) error {
	if now < refreshAt {
		return nil
	}
	return service.refreshMarked()
}

func (service *SessionTokenService) renewUnlessRefreshing() error {
	if service.refreshing {
		return nil
	}
	return service.refreshMarked()
}

func (service *SessionTokenService) switchUnlessRefreshing(domainId string) (Session, error) {
	if service.refreshing {
		return service.session, errors.New("a session refresh is running")
	}
	return service.switchMarked(domainId)
}

func (service *SessionTokenService) keepOrForget(session Session, failure error) error {
	service.forgetWhenEnded(service.statusOf(failure))
	return service.keepUnlessFailed(session, failure)
}

func (service *SessionTokenService) forgetWhenEnded(status int) {
	if !service.endedStatuses()[status] {
		return
	}
	service.forget()
}

func (service *SessionTokenService) keepUnlessFailed(session Session, failure error) error {
	if failure != nil {
		return failure
	}
	return service.keep(session)
}

func (service *SessionTokenService) keep(session Session) error {
	now, failure := service.clock.NowMs()
	service.key = session.SessionKey
	service.session = session
	service.tokens = SessionTokens{
		AccessToken: session.AccessToken,
		ExpiresAtMs: now + session.ExpiresIn*service.millisecondsPerSecond(),
	}
	return failure
}

func (service *SessionTokenService) forget() {
	service.key = ""
	service.session = Session{}
	service.tokens = SessionTokens{}
}

func (service *SessionTokenService) statusOf(failure error) int {
	if failure == nil {
		return 0
	}
	return BehaviorErrorStatus(failure)
}

func (service *SessionTokenService) endedStatuses() map[int]bool {
	return map[int]bool{401: true, 403: true}
}

func (service *SessionTokenService) forbidden() int {
	return 403
}

func (service *SessionTokenService) defaultDomain() string {
	return ""
}

func (service *SessionTokenService) refreshMarginMs() float64 {
	return 15000
}

func (service *SessionTokenService) millisecondsPerSecond() float64 {
	return 1000
}
