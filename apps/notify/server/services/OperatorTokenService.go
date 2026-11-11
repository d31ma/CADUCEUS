import (
	"strings"

	"mesail.dev/packages/oidc"
)

// @Service
type OperatorTokenService struct{}

func (tokens *OperatorTokenService) Token() (string, error) {
	authorization, failure := oidc.ServiceTokensFactory{}.Configured().Authorization(tokens.realm())
	return strings.TrimPrefix(authorization, tokens.bearerPrefix()), failure
}

func (tokens *OperatorTokenService) realm() string {
	return "mesail-operators"
}

func (tokens *OperatorTokenService) bearerPrefix() string {
	return "Bearer "
}
