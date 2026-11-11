import (
	"net/http"
	"os"

	"mesail.dev/packages/oidc"
	"mesail.dev/packages/realtime"
)

// @IdentityProvider
type TokenIdentityProvider struct {
}

func (provider *TokenIdentityProvider) Identify(request *http.Request) *Identity {
	_, fromGateway := request.Header[http.CanonicalHeaderKey(provider.gatewayHeader())]
	gateway := provider.gateway(request.Header.Get(provider.gatewayHeader()))
	member := provider.member(request.Header.Get("Authorization"))
	return provider.chosen(fromGateway, gateway, member)
}

func (provider *TokenIdentityProvider) gateway(secret string) *Identity {
	if !(realtime.DeskFactory{Environment: os.Getenv}).GatewayCheck().Allowed(secret) {
		return nil
	}
	return &Identity{User: "gateway", Roles: []string{"Gateway"}}
}

func (provider *TokenIdentityProvider) member(authorization string) *Identity {
	caller, failure := oidc.Configured().Identify(authorization)
	return provider.identity(caller, failure)
}

func (provider *TokenIdentityProvider) chosen(fromGateway bool, gateway *Identity, member *Identity) *Identity {
	if fromGateway {
		return gateway
	}
	return member
}

func (provider *TokenIdentityProvider) identity(caller oidc.Caller, failure error) *Identity {
	if failure != nil {
		return nil
	}
	return &Identity{User: caller.DomainId + "/" + caller.MemberId, Roles: caller.Roles}
}

func (provider *TokenIdentityProvider) gatewayHeader() string {
	return "X-Realtime-Gateway"
}
