import (
	"errors"

	"mesail.dev/packages/login"
	"mesail.dev/packages/oidc"
)

// @Service
type TokenService struct {
}

func (service *TokenService) KeySet() (KeySet, error) {
	set, failed := oidc.TokenSignerFactory{}.Configured().KeySet()
	return service.keySet(set), service.keySetFailure(failed)
}

func (service *TokenService) ServiceToken(request ServiceTokenRequest) (ServiceToken, error) {
	issued, failed := login.SessionsFactory{}.ConfiguredServiceTokens().Issue(request.ClientId, request.ClientSecret, request.DomainId)
	return ServiceToken{AccessToken: issued.AccessToken, ExpiresIn: issued.ExpiresIn}, service.serviceTokenFailure(failed)
}

func (service *TokenService) keySet(set oidc.KeySet) KeySet {
	keys := []PublicKey{}
	for _, key := range set.Keys {
		keys = append(keys, PublicKey{KeyType: key.KeyType, Curve: key.Curve, X: key.X, Y: key.Y, KeyId: key.KeyId, Algorithm: key.Algorithm, Use: key.Use})
	}
	return KeySet{Keys: keys}
}

func (service *TokenService) keySetFailure(failed error) error {
	if failed != nil {
		return Unavailable{}.SigningKeys()
	}
	return nil
}

func (service *TokenService) serviceTokenFailure(failed error) error {
	switch {
	case failed == nil:
		return nil
	case errors.Is(failed, login.InvalidDomain{}):
		return BadRequest{}.InvalidDomain()
	case errors.Is(failed, oidc.SignInRefused{}):
		return Unauthenticated{}.ClientRefused()
	}
	return Unavailable{}.ServiceTokens()
}
