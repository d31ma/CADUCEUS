// @Service
type TokenCheckService struct{}

func (service *TokenCheckService) Grant(token string) (AuthGrant, error) {
	return AuthGrant{}, &Unauthenticated{}
}
