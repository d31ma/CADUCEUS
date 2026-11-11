import (
	"net/http"

	"mesail.dev/packages/oidc"
)

// @IdentityProvider
type OidcIdentityProvider struct {
}

func (provider *OidcIdentityProvider) Identify(request *http.Request) *Identity {
	caller, err := oidc.Configured().Identify(request.Header.Get("Authorization"))
	return provider.identity(caller, err)
}

func (provider *OidcIdentityProvider) identity(caller oidc.Caller, err error) *Identity {
	if err != nil {
		return nil
	}
	return &Identity{User: caller.DomainId + "/" + caller.MemberId, Roles: caller.Roles}
}
