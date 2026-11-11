// @Service
type ConversationTokenService struct {
	// @Needs
	session *SessionService
	// @Needs
	accounts *SessionAccountService
}

func (service *ConversationTokenService) WithRenewal(call func(authorization string) error) error {
	return service.retryWhenUnauthorized(call, service.firstCall(call))
}

func (service *ConversationTokenService) AccessToken() (string, error) {
	return service.session.AccessToken()
}

func (service *ConversationTokenService) DomainId() string {
	return service.accounts.CurrentDomain
}

func (service *ConversationTokenService) MyAddress() string {
	return service.accounts.Current().Address
}

func (service *ConversationTokenService) authorization() (string, error) {
	accessToken, failure := service.session.AccessToken()
	return service.bearerPrefix() + accessToken, failure
}

func (service *ConversationTokenService) firstCall(call func(authorization string) error) error {
	authorization, failure := service.authorization()
	if failure != nil {
		return failure
	}
	return call(authorization)
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

func (service *ConversationTokenService) unauthorizedStatus() int {
	return 401
}

func (service *ConversationTokenService) bearerPrefix() string {
	return "Bearer "
}
