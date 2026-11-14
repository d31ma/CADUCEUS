import (
	"net/http"

	"mesail.dev/packages/oidc"
)

// @IdentityProvider
type TokenIdentityProvider struct {
}

func (provider *TokenIdentityProvider) Identify(request *http.Request) *Identity {
	caller, failure := oidc.Configured().Identify(request.Header.Get("Authorization"))
	return provider.identity(caller, failure)
}

func (provider *TokenIdentityProvider) identity(caller oidc.Caller, failure error) *Identity {
	if failure != nil {
		return nil
	}
	return &Identity{User: caller.DomainId + "/" + caller.MemberId, Roles: caller.Roles}
}
