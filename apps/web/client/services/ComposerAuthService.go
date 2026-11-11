// @Service
type ComposerAuthService struct {
	// @Needs
	session *SessionService
	// @Needs
	accounts *SessionAccountService
}

func (auth *ComposerAuthService) Authorization() (string, error) {
	token, failure := auth.session.AccessToken()
	return auth.bearer(token), failure
}

func (auth *ComposerAuthService) Renewed() (string, error) {
	token, failure := auth.session.Renew()
	return auth.bearer(token), failure
}

func (auth *ComposerAuthService) DomainId() string {
	return auth.accounts.CurrentDomain
}

func (auth *ComposerAuthService) Address() string {
	return auth.accounts.Current().Address
}

func (auth *ComposerAuthService) bearer(token string) string {
	return "Bearer " + token
}
