// @Service
type SessionTokenService struct {
	tokens map[string]SessionTokens
	// @Needs
	identity *IdentityClient
	// @Needs
	clock Clock
}

func (service *SessionTokenService) Exchange(pending SessionPending, code string, redirectUri string) error {
	request := SessionTokenRequest{DomainId: pending.DomainId, Code: code, CodeVerifier: pending.Verifier, RedirectUri: redirectUri}
	answer, failure := BehaviorsAnswer[SessionTokenAnswer](service.identity.Token(request))
	return service.keepUnlessFailed(pending.DomainId, answer, failure)
}

func (service *SessionTokenService) AccessToken(domainId string) (string, error) {
	failure := service.refreshWhenDue(domainId)
	return service.saved(domainId).AccessToken, failure
}

func (service *SessionTokenService) IdToken(domainId string) string {
	return service.saved(domainId).IdToken
}

func (service *SessionTokenService) Forget(domainId string) {
	delete(service.store(), domainId)
}

func (service *SessionTokenService) refresh(domainId string) error {
	request := SessionRefreshRequest{DomainId: domainId, RefreshToken: service.saved(domainId).RefreshToken}
	answer, failure := BehaviorsAnswer[SessionTokenAnswer](service.identity.Refresh(request))
	service.Forget(domainId)
	return service.keepUnlessFailed(domainId, answer, failure)
}

func (service *SessionTokenService) keepUnlessFailed(domainId string, answer SessionTokenAnswer, failure error) error {
	if failure != nil {
		return failure
	}
	return service.keep(domainId, answer)
}

func (service *SessionTokenService) keep(domainId string, answer SessionTokenAnswer) error {
	now, failure := service.clock.NowMs()
	service.store()[domainId] = SessionTokens{
		AccessToken:  answer.AccessToken,
		RefreshToken: answer.RefreshToken,
		IdToken:      answer.IdToken,
		ExpiresAtMs:  now + answer.ExpiresIn*service.millisecondsPerSecond(),
	}
	return failure
}

func (service *SessionTokenService) refreshWhenDue(domainId string) error {
	now, failure := service.clock.NowMs()
	if failure != nil || now < service.saved(domainId).ExpiresAtMs-service.refreshMarginMs() {
		return failure
	}
	return service.refresh(domainId)
}

func (service *SessionTokenService) saved(domainId string) SessionTokens {
	return service.store()[domainId]
}

func (service *SessionTokenService) store() map[string]SessionTokens {
	if service.tokens == nil {
		service.tokens = map[string]SessionTokens{}
	}
	return service.tokens
}

func (service *SessionTokenService) refreshMarginMs() float64 {
	return 15000
}

func (service *SessionTokenService) millisecondsPerSecond() float64 {
	return 1000
}
