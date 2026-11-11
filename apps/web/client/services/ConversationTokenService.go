// @Service
type ConversationTokenService struct {
	// @Needs
	session *SessionService
	// @Needs
	accounts *SessionAccountService
}

func (service *ConversationTokenService) Authorization() (string, error) {
	accessToken, failure := service.session.AccessToken()
	return service.bearerPrefix() + accessToken, failure
}

func (service *ConversationTokenService) AccessToken() (string, error) {
	return service.session.AccessToken()
}

func (service *ConversationTokenService) WithRenewal(call func(authorization string) error) error {
	authorization, failure := service.Authorization()
	if failure != nil {
		return failure
	}
	return service.retryWhenUnauthorized(call, call(authorization))
}

func (service *ConversationTokenService) DomainId() string {
	return service.accounts.CurrentDomain
}

func (service *ConversationTokenService) MyAddress() string {
	return service.accounts.Current().Address
}

func (service *ConversationTokenService) retryWhenUnauthorized(call func(authorization string) error, failure error) error {
	if BehaviorErrorStatus(failure) != service.unauthorizedStatus() {
		return failure
	}
	return call(service.renewedAuthorization())
}

func (service *ConversationTokenService) renewedAuthorization() string {
	accessToken, _ := service.session.Renew()
	return service.bearerPrefix() + accessToken
}

func (service *ConversationTokenService) bearerPrefix() string {
	return "Bearer "
}

func (service *ConversationTokenService) unauthorizedStatus() int {
	return 401
}
