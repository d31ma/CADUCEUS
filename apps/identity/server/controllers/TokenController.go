import (
	"errors"

	"mesail.dev/packages/login"
	"mesail.dev/packages/oidc"
)

// @RequestRoute("/api/v1/identity")
// @Public
type TokenController struct {
}

// @GetRoute("/jwks")
func (controller *TokenController) GetJwks() (KeySet, error) {
	set, failed := oidc.TokenSignerFactory{}.Configured().KeySet()
	return controller.keySet(set), controller.failure(failed)
}

// @PostRoute("/service-token")
func (controller *TokenController) PostServiceToken(
	// @FromBody
	request ServiceTokenRequest,
) (ServiceToken, error) {
	issued, failed := login.SessionsFactory{}.ConfiguredServiceTokens().Issue(request.ClientId, request.ClientSecret, request.DomainId)
	return ServiceToken{AccessToken: issued.AccessToken, ExpiresIn: issued.ExpiresIn}, controller.failure(failed)
}

func (controller *TokenController) keySet(set oidc.KeySet) KeySet {
	keys := []PublicKey{}
	for _, key := range set.Keys {
		keys = append(keys, PublicKey{KeyType: key.KeyType, Curve: key.Curve, X: key.X, Y: key.Y, KeyId: key.KeyId, Algorithm: key.Algorithm, Use: key.Use})
	}
	return KeySet{Keys: keys}
}

func (controller *TokenController) failure(failed error) error {
	switch {
	case failed == nil:
		return nil
	case errors.Is(failed, login.InvalidDomain{}):
		return &InvalidRequest{}
	case errors.Is(failed, oidc.SignInRefused{}):
		return &Unauthenticated{}
	}
	return &Unavailable{}
}
