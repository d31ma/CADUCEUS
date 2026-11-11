// @Service
type NotificationTokenService struct {
	// @Needs
	tokens *SessionTokenService
	// @Needs
	accounts *SessionAccountService
	// @Needs
	problems *NotificationProblemService
}

func (service *NotificationTokenService) Authorization() (string, error) {
	return service.authorizationFor(service.accounts.CurrentDomain)
}

func (service *NotificationTokenService) DomainId() string {
	return service.accounts.CurrentDomain
}

func (service *NotificationTokenService) Account() SessionAccount {
	return service.accounts.Current()
}

func (service *NotificationTokenService) authorizationFor(domainId string) (string, error) {
	if domainId == "" {
		return "", service.problems.SignedOut()
	}
	accessToken, failure := service.tokens.AccessToken(domainId)
	return service.bearerPrefix() + accessToken, failure
}

func (service *NotificationTokenService) bearerPrefix() string {
	return "Bearer "
}
