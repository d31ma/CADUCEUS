import (
	"errors"
	"maps"
	"sort"
)

// @Service
type SessionTokenService struct {
	keys   map[string]string
	tokens map[string]SessionTokens
	// @Needs
	identity *IdentityClient
	// @Needs
	clock Clock
}

func (service *SessionTokenService) SignIn(request SessionSignInRequest) (Me, error) {
	session, failure := BehaviorsAnswer[Session](service.identity.SignIn(request))
	return session.Me, service.keepUnlessFailed(session, failure)
}

func (service *SessionTokenService) Restore(keys map[string]string) []Me {
	service.keys = maps.Clone(keys)
	return service.restoreAll(service.domains())
}

func (service *SessionTokenService) SavedKeys() map[string]string {
	return maps.Clone(service.keys)
}

func (service *SessionTokenService) AccessToken(domainId string) (string, error) {
	failure := service.refreshWhenDue(domainId)
	return service.tokens[domainId].AccessToken, failure
}

func (service *SessionTokenService) Renew(domainId string) error {
	_, failure := service.refresh(domainId)
	return failure
}

func (service *SessionTokenService) ChangePassword(domainId string, change SessionPasswordChange) error {
	accessToken, failure := service.AccessToken(domainId)
	change.SessionKey = service.keys[domainId]
	session, answerFailure := BehaviorsAnswer[Session](service.identity.ChangePassword("Bearer "+accessToken, change))
	return service.keepUnlessFailed(session, errors.Join(failure, answerFailure))
}

func (service *SessionTokenService) SignOut(domainId string) error {
	_, failure := service.identity.SignOut(SessionRefreshRequest{SessionKey: service.keys[domainId]})
	service.forget(domainId)
	return failure
}

func (service *SessionTokenService) forget(domainId string) {
	delete(service.keys, domainId)
	delete(service.tokens, domainId)
}

func (service *SessionTokenService) restoreAll(domainIds []string) []Me {
	signedIn := []Me{}
	for _, domainId := range domainIds {
		signedIn = service.addRestored(signedIn, domainId)
	}
	return signedIn
}

func (service *SessionTokenService) addRestored(signedIn []Me, domainId string) []Me {
	me, failure := service.refresh(domainId)
	if failure != nil {
		return signedIn
	}
	return append(signedIn, me)
}

func (service *SessionTokenService) refresh(domainId string) (Me, error) {
	request := SessionRefreshRequest{SessionKey: service.keys[domainId]}
	session, failure := BehaviorsAnswer[Session](service.identity.Refresh(request))
	service.forgetWhenFailed(domainId, failure)
	return session.Me, service.keepUnlessFailed(session, failure)
}

func (service *SessionTokenService) refreshWhenDue(domainId string) error {
	now, failure := service.clock.NowMs()
	if failure != nil || now < service.tokens[domainId].ExpiresAtMs-service.refreshMarginMs() {
		return failure
	}
	_, failure = service.refresh(domainId)
	return failure
}

func (service *SessionTokenService) keepUnlessFailed(session Session, failure error) error {
	if failure != nil {
		return failure
	}
	return service.keep(session)
}

func (service *SessionTokenService) keep(session Session) error {
	now, failure := service.clock.NowMs()
	domainId := session.Me.DomainId
	service.makeKeys()
	service.makeTokens()
	service.keys[domainId] = session.SessionKey
	service.tokens[domainId] = SessionTokens{
		AccessToken: session.AccessToken,
		ExpiresAtMs: now + session.ExpiresIn*service.millisecondsPerSecond(),
	}
	return failure
}

func (service *SessionTokenService) forgetWhenFailed(domainId string, failure error) {
	if failure == nil {
		return
	}
	service.forget(domainId)
}

func (service *SessionTokenService) domains() []string {
	domainIds := []string{}
	for domainId := range service.keys {
		domainIds = append(domainIds, domainId)
	}
	sort.Strings(domainIds)
	return domainIds
}

func (service *SessionTokenService) makeKeys() {
	if service.keys == nil {
		service.keys = map[string]string{}
	}
}

func (service *SessionTokenService) makeTokens() {
	if service.tokens == nil {
		service.tokens = map[string]SessionTokens{}
	}
}

func (service *SessionTokenService) refreshMarginMs() float64 {
	return 15000
}

func (service *SessionTokenService) millisecondsPerSecond() float64 {
	return 1000
}
