package oidc

import "fmt"

type UnknownIssuer struct {
	Issuer string
}

func (unknown UnknownIssuer) Error() string {
	return fmt.Sprintf("the issuer %q is not a Keycloak realm URL", unknown.Issuer)
}
