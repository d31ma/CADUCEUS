import "sort"

// @Service
type SessionTokenService struct {
	// @SessionState
	Keys   map[string]string
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

func (service *SessionTokenService) Restore() []Me {
	return service.restoreAll(service.domains())
}

func (service *SessionTokenService) AccessToken(domainId string) (string, error) {
	failure := service.refreshWhenDue(domainId)
	return service.tokens[domainId].AccessToken, failure
}

func (service *SessionTokenService) SignOut(domainId string) error {
	_, failure := service.identity.SignOut(SessionRefreshRequest{SessionKey: service.Keys[domainId]})
	service.Forget(domainId)
	return failure
}

func (service *SessionTokenService) Forget(domainId string) {
	delete(service.Keys, domainId)
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
	request := SessionRefreshRequest{SessionKey: service.Keys[domainId]}
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
	service.Keys[domainId] = session.SessionKey
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
	service.Forget(domainId)
}

func (service *SessionTokenService) domains() []string {
	domainIds := []string{}
	for domainId := range service.Keys {
		domainIds = append(domainIds, domainId)
	}
	sort.Strings(domainIds)
	return domainIds
}

func (service *SessionTokenService) makeKeys() {
	if service.Keys == nil {
		service.Keys = map[string]string{}
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
