import (
	"strings"

	"mesail.dev/packages/oidc"
)

type DomainServiceToken struct {
	DomainId string
}

func (token DomainServiceToken) Token() (string, error) {
	authorization, failure := oidc.ServiceTokensFactory{}.Configured().Authorization(token.DomainId)
	return strings.TrimPrefix(authorization, "Bearer "), failure
}
