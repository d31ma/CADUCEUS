// @Service
type ComposerAuthService struct {
	// @Needs
	tokens *SessionTokenService
	// @Needs
	accounts *SessionAccountService
}

func (auth *ComposerAuthService) Authorization() (string, error) {
	token, failure := auth.tokens.AccessToken(auth.accounts.CurrentDomain)
	return "Bearer " + token, failure
}

func (auth *ComposerAuthService) DomainId() string {
	return auth.accounts.CurrentDomain
}

func (auth *ComposerAuthService) Address() string {
	return auth.accounts.Current().Address
}
