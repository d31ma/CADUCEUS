import (
	"strings"

	"mesail.dev/packages/oidc"
)

type RealmServiceToken struct {
	Realm string
}

func (token RealmServiceToken) Token() (string, error) {
	authorization, failure := oidc.ServiceTokensFactory{}.Configured().Authorization(token.Realm)
	return strings.TrimPrefix(authorization, "Bearer "), failure
}
