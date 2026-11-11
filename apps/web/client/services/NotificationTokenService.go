// @Service
type NotificationTokenService struct {
	// @Needs
	session *SessionService
	// @Needs
	accounts *SessionAccountService
	// @Needs
	problems *NotificationProblemService
}

func (service *NotificationTokenService) Send(request func(authorization string) (BehaviorValue, error)) (BehaviorValue, error) {
	answer, failure := service.sendWith(request, service.authorization)
	return service.retryWhenEnded(request, answer, failure)
}

func (service *NotificationTokenService) DomainId() string {
	return service.accounts.CurrentDomain
}

func (service *NotificationTokenService) Account() SessionAccount {
	return service.accounts.Current()
}

func (service *NotificationTokenService) retryWhenEnded(request func(string) (BehaviorValue, error), answer BehaviorValue, failure error) (BehaviorValue, error) {
	if BehaviorErrorStatus(failure) != service.unauthenticated() {
		return answer, failure
	}
	return service.sendWith(request, service.renewed)
}

func (service *NotificationTokenService) sendWith(request func(string) (BehaviorValue, error), authorize func() (string, error)) (BehaviorValue, error) {
	authorization, failure := authorize()
	return service.sendWhenReady(request, authorization, failure)
}

func (service *NotificationTokenService) sendWhenReady(request func(string) (BehaviorValue, error), authorization string, failure error) (BehaviorValue, error) {
	if failure != nil {
		return nil, failure
	}
	return request(authorization)
}

func (service *NotificationTokenService) authorization() (string, error) {
	return service.authorizationFor(service.accounts.CurrentDomain)
}

func (service *NotificationTokenService) authorizationFor(domainId string) (string, error) {
	if domainId == "" {
		return "", service.problems.SignedOut()
	}
	accessToken, failure := service.session.AccessToken()
	return service.bearerPrefix() + accessToken, failure
}

func (service *NotificationTokenService) renewed() (string, error) {
	accessToken, failure := service.session.Renew()
	return service.bearerPrefix() + accessToken, failure
}

func (service *NotificationTokenService) bearerPrefix() string {
	return "Bearer "
}

func (service *NotificationTokenService) unauthenticated() int {
	return 401
}
