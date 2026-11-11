import (
	"strings"

	"mesail.dev/packages/oidc"
)

// @Service
type ServiceTokenService struct{}

func (tokens *ServiceTokenService) TokenFor(domainId string) (string, error) {
	authorization, failure := oidc.ServiceTokensFactory{}.Configured().Authorization(domainId)
	return strings.TrimPrefix(authorization, tokens.bearerPrefix()), failure
}

func (tokens *ServiceTokenService) bearerPrefix() string {
	return "Bearer "
}
