// @Service
type ConversationTokenService struct {
	// @Needs
	tokens *SessionTokenService
	// @Needs
	accounts *SessionAccountService
}

func (service *ConversationTokenService) Authorization() (string, error) {
	accessToken, failure := service.tokens.AccessToken(service.accounts.CurrentDomain)
	return service.bearerPrefix() + accessToken, failure
}

func (service *ConversationTokenService) AccessToken() (string, error) {
	return service.tokens.AccessToken(service.accounts.CurrentDomain)
}

func (service *ConversationTokenService) DomainId() string {
	return service.accounts.CurrentDomain
}

func (service *ConversationTokenService) MyAddress() string {
	return service.accounts.Current().Address
}

func (service *ConversationTokenService) bearerPrefix() string {
	return "Bearer "
}
